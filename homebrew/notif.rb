class Notif < Formula
  desc "Notification center for Claude Code sessions"
  homepage "https://github.com/filipexyz/notif"
  version "0.1.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/filipexyz/notif/releases/download/v#{version}/notif-aarch64-apple-darwin.tar.gz"
      sha256 "PLACEHOLDER_DARWIN_ARM64_SHA"
    end
    on_intel do
      url "https://github.com/filipexyz/notif/releases/download/v#{version}/notif-x86_64-apple-darwin.tar.gz"
      sha256 "PLACEHOLDER_DARWIN_AMD64_SHA"
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/filipexyz/notif/releases/download/v#{version}/notif-x86_64-unknown-linux-gnu.tar.gz"
      sha256 "PLACEHOLDER_LINUX_AMD64_SHA"
    end
  end

  def install
    bin.install "notif"
  end

  test do
    assert_match "notif", shell_output("#{bin}/notif --version")
  end
end
