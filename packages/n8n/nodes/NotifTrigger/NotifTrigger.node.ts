import type {
	IHookFunctions,
	IWebhookFunctions,
	INodeType,
	INodeTypeDescription,
	IWebhookResponseData,
} from 'n8n-workflow';
import { createHmac, timingSafeEqual } from 'crypto';

export class NotifTrigger implements INodeType {
	description: INodeTypeDescription = {
		displayName: 'Notif Trigger',
		name: 'notifTrigger',
		icon: 'file:notif.svg',
		group: ['trigger'],
		version: 1,
		subtitle: '={{$parameter["topics"]}}',
		description: 'Starts the workflow when notif.sh events are received',
		defaults: {
			name: 'Notif Trigger',
		},
		inputs: [],
		outputs: ['main'],
		credentials: [
			{
				name: 'notifApi',
				required: true,
			},
		],
		webhooks: [
			{
				name: 'default',
				httpMethod: 'POST',
				responseMode: 'onReceived',
				path: 'webhook',
			},
		],
		properties: [
			{
				displayName: 'Topics',
				name: 'topics',
				type: 'string',
				default: '',
				required: true,
				placeholder: 'orders.*, leads.new',
				description: 'Comma-separated topic patterns to subscribe to. Use * for single segment wildcard, > for all remaining segments.',
			},
		],
	};

	webhookMethods = {
		default: {
			async checkExists(this: IHookFunctions): Promise<boolean> {
				const webhookData = this.getWorkflowStaticData('node');

				if (!webhookData.webhookId) {
					return false;
				}

				const credentials = await this.getCredentials('notifApi');
				const serverUrl = credentials.serverUrl as string;

				try {
					await this.helpers.request({
						method: 'GET',
						url: `${serverUrl}/api/v1/webhooks/${webhookData.webhookId}`,
						headers: {
							Authorization: `Bearer ${credentials.apiKey}`,
						},
						json: true,
					});
					return true;
				} catch (error) {
					if ((error as { statusCode?: number }).statusCode === 404) {
						delete webhookData.webhookId;
						delete webhookData.webhookSecret;
						return false;
					}
					throw error;
				}
			},

			async create(this: IHookFunctions): Promise<boolean> {
				const webhookUrl = this.getNodeWebhookUrl('default') as string;
				const webhookData = this.getWorkflowStaticData('node');
				const credentials = await this.getCredentials('notifApi');
				const serverUrl = credentials.serverUrl as string;

				const topicsParam = this.getNodeParameter('topics') as string;
				const topics = topicsParam.split(',').map((t) => t.trim()).filter(Boolean);

				if (topics.length === 0) {
					throw new Error('At least one topic is required');
				}

				const response = await this.helpers.request({
					method: 'POST',
					url: `${serverUrl}/api/v1/webhooks`,
					headers: {
						Authorization: `Bearer ${credentials.apiKey}`,
						'Content-Type': 'application/json',
					},
					body: {
						url: webhookUrl,
						topics,
					},
					json: true,
				});

				if (!response.id) {
					throw new Error('Failed to create webhook: no ID returned');
				}

				webhookData.webhookId = response.id;
				webhookData.webhookSecret = response.secret;

				return true;
			},

			async delete(this: IHookFunctions): Promise<boolean> {
				const webhookData = this.getWorkflowStaticData('node');

				if (!webhookData.webhookId) {
					return true;
				}

				const credentials = await this.getCredentials('notifApi');
				const serverUrl = credentials.serverUrl as string;

				try {
					await this.helpers.request({
						method: 'DELETE',
						url: `${serverUrl}/api/v1/webhooks/${webhookData.webhookId}`,
						headers: {
							Authorization: `Bearer ${credentials.apiKey}`,
						},
						json: true,
					});
				} catch (error) {
					if ((error as { statusCode?: number }).statusCode !== 404) {
						throw error;
					}
				}

				delete webhookData.webhookId;
				delete webhookData.webhookSecret;

				return true;
			},
		},
	};

	async webhook(this: IWebhookFunctions): Promise<IWebhookResponseData> {
		const req = this.getRequestObject();
		const webhookData = this.getWorkflowStaticData('node');
		const secret = webhookData.webhookSecret as string;

		// Verify HMAC signature if secret is available
		if (secret) {
			const signature = req.headers['x-notif-signature'] as string;
			const body = JSON.stringify(req.body);

			if (!signature || !verifySignature(body, signature, secret)) {
				return {
					webhookResponse: 'Invalid signature',
					workflowData: [],
				};
			}
		}

		const event = req.body as {
			id: string;
			topic: string;
			data: Record<string, unknown>;
			timestamp: string;
		};

		return {
			workflowData: [
				this.helpers.returnJsonArray({
					id: event.id,
					topic: event.topic,
					data: event.data,
					timestamp: event.timestamp,
				}),
			],
		};
	}
}

function verifySignature(payload: string, signature: string, secret: string): boolean {
	const expected =
		'sha256=' + createHmac('sha256', secret).update(payload).digest('hex');

	try {
		return timingSafeEqual(Buffer.from(signature), Buffer.from(expected));
	} catch {
		return false;
	}
}
