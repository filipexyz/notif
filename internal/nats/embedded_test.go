package nats

import (
	"context"
	"testing"
	"time"

	"github.com/filipexyz/notif/internal/accounts"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func TestEmbeddedBasic(t *testing.T) {
	srv, err := StartEmbedded(EmbeddedConfig{
		StoreDir: t.TempDir(),
		Port:     -1,
	})
	if err != nil {
		t.Fatalf("start embedded: %v", err)
	}
	defer srv.Shutdown()

	// Connect a plain client
	nc, err := nats.Connect(srv.ClientURL())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer nc.Close()

	// Test pub/sub
	received := make(chan *nats.Msg, 1)
	sub, err := nc.Subscribe("test.hello", func(msg *nats.Msg) {
		received <- msg
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()
	nc.Flush()

	if err := nc.Publish("test.hello", []byte("world")); err != nil {
		t.Fatalf("publish: %v", err)
	}
	nc.Flush()

	select {
	case msg := <-received:
		if string(msg.Data) != "world" {
			t.Errorf("expected 'world', got %q", string(msg.Data))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for message")
	}

	// Test JetStream
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}

	ctx := context.Background()
	stream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     "TEST_EMBEDDED",
		Subjects: []string{"test.js.>"},
		Storage:  jetstream.MemoryStorage,
	})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}

	if _, err := js.Publish(ctx, "test.js.event", []byte("jetstream-works")); err != nil {
		t.Fatalf("js publish: %v", err)
	}

	consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		FilterSubject: "test.js.>",
		DeliverPolicy: jetstream.DeliverAllPolicy,
		AckPolicy:     jetstream.AckNonePolicy,
	})
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}

	msgs, err := consumer.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	var gotJS bool
	for msg := range msgs.Messages() {
		if string(msg.Data()) == "jetstream-works" {
			gotJS = true
		}
	}
	if !gotJS {
		t.Fatal("JetStream message not received")
	}
}

func TestEmbeddedMultiAccount(t *testing.T) {
	// Generate operator and system account keys
	operatorKP, err := accounts.GenerateOperatorKey()
	if err != nil {
		t.Fatalf("generate operator key: %v", err)
	}
	operatorPub, err := operatorKP.PublicKey()
	if err != nil {
		t.Fatalf("get operator public key: %v", err)
	}

	systemKP, err := accounts.GenerateAccountKey()
	if err != nil {
		t.Fatalf("generate system account key: %v", err)
	}
	systemPub, err := systemKP.PublicKey()
	if err != nil {
		t.Fatalf("get system account public key: %v", err)
	}

	srv, err := StartEmbedded(EmbeddedConfig{
		StoreDir:               t.TempDir(),
		Port:                   -1,
		OperatorPublicKey:      operatorPub,
		SystemAccountPublicKey: systemPub,
	})
	if err != nil {
		t.Fatalf("start embedded: %v", err)
	}
	defer srv.Shutdown()

	// Connect system account
	sysUserKP, err := accounts.GenerateUserKey()
	if err != nil {
		t.Fatalf("generate system user key: %v", err)
	}
	sysUserPub, err := sysUserKP.PublicKey()
	if err != nil {
		t.Fatalf("get system user public key: %v", err)
	}

	// Build system account JWT and push it
	sysAccountClaims := jwt.NewAccountClaims(systemPub)
	sysAccountClaims.Name = "SYS"
	sysAccountClaims.Limits.Conn = -1
	sysAccountClaims.Limits.Data = -1
	sysAccountClaims.Limits.Payload = -1
	sysAccountClaims.Limits.Subs = -1
	sysAccountJWT, err := sysAccountClaims.Encode(operatorKP)
	if err != nil {
		t.Fatalf("encode system account JWT: %v", err)
	}

	sysUserClaims := jwt.NewUserClaims(sysUserPub)
	sysUserClaims.IssuerAccount = systemPub
	sysUserClaims.Name = "test-system"
	sysUserJWT, err := sysUserClaims.Encode(systemKP)
	if err != nil {
		t.Fatalf("encode system user JWT: %v", err)
	}

	sysUserSeed, err := sysUserKP.Seed()
	if err != nil {
		t.Fatalf("get system user seed: %v", err)
	}

	sysNC, err := nats.Connect(srv.ClientURL(),
		nats.UserJWTAndSeed(sysUserJWT, string(sysUserSeed)),
	)
	if err != nil {
		t.Fatalf("connect system: %v", err)
	}
	defer sysNC.Close()

	// Push system account JWT
	resp, err := sysNC.Request("$SYS.REQ.CLAIMS.UPDATE", []byte(sysAccountJWT), 5*time.Second)
	if err != nil {
		t.Fatalf("push system account JWT: %v", err)
	}
	t.Logf("system account JWT pushed: %s", string(resp.Data))

	// Create a regular account and connect
	accountKP, err := accounts.GenerateAccountKey()
	if err != nil {
		t.Fatalf("generate account key: %v", err)
	}
	accountPub, err := accountKP.PublicKey()
	if err != nil {
		t.Fatalf("get account public key: %v", err)
	}

	accountClaims := jwt.NewAccountClaims(accountPub)
	accountClaims.Name = "test-org"
	accountClaims.Limits.Conn = -1
	accountClaims.Limits.Subs = -1
	accountClaims.Limits.Data = -1
	accountClaims.Limits.Payload = -1
	accountClaims.Limits.JetStreamLimits = jwt.JetStreamLimits{
		MemoryStorage: -1,
		DiskStorage:   -1,
		Streams:       -1,
		Consumer:      -1,
	}
	accountJWT, err := accountClaims.Encode(operatorKP)
	if err != nil {
		t.Fatalf("encode account JWT: %v", err)
	}

	// Push account JWT
	resp, err = sysNC.Request("$SYS.REQ.CLAIMS.UPDATE", []byte(accountJWT), 5*time.Second)
	if err != nil {
		t.Fatalf("push account JWT: %v", err)
	}
	t.Logf("account JWT pushed: %s", string(resp.Data))

	// Connect as regular account user
	userKP, err := accounts.GenerateUserKey()
	if err != nil {
		t.Fatalf("generate user key: %v", err)
	}
	userPub, err := userKP.PublicKey()
	if err != nil {
		t.Fatalf("get user public key: %v", err)
	}

	userClaims := jwt.NewUserClaims(userPub)
	userClaims.IssuerAccount = accountPub
	userClaims.Name = "test-user"
	userJWT, err := userClaims.Encode(accountKP)
	if err != nil {
		t.Fatalf("encode user JWT: %v", err)
	}

	userSeed, err := userKP.Seed()
	if err != nil {
		t.Fatalf("get user seed: %v", err)
	}

	userNC, err := nats.Connect(srv.ClientURL(),
		nats.UserJWTAndSeed(userJWT, string(userSeed)),
	)
	if err != nil {
		t.Fatalf("connect user: %v", err)
	}
	defer userNC.Close()

	// Verify the user connection works: pub/sub within account
	received := make(chan *nats.Msg, 1)
	sub, err := userNC.Subscribe("account.test", func(msg *nats.Msg) {
		received <- msg
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()
	userNC.Flush()

	if err := userNC.Publish("account.test", []byte("isolated")); err != nil {
		t.Fatalf("publish: %v", err)
	}
	userNC.Flush()

	select {
	case msg := <-received:
		if string(msg.Data) != "isolated" {
			t.Errorf("expected 'isolated', got %q", string(msg.Data))
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for message in account")
	}

	// Verify JetStream works in the account
	js, err := jetstream.New(userNC)
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}

	ctx := context.Background()
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     "TEST_ACCOUNT",
		Subjects: []string{"events.>"},
		Storage:  jetstream.MemoryStorage,
	})
	if err != nil {
		t.Fatalf("create stream in account: %v", err)
	}

	if _, err := js.Publish(ctx, "events.test", []byte("account-js")); err != nil {
		t.Fatalf("js publish in account: %v", err)
	}
}
