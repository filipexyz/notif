import type {
	IExecuteFunctions,
	INodeExecutionData,
	INodeType,
	INodeTypeDescription,
} from 'n8n-workflow';

export class Notif implements INodeType {
	description: INodeTypeDescription = {
		displayName: 'Notif',
		name: 'notif',
		icon: 'file:notif.svg',
		group: ['output'],
		version: 1,
		subtitle: '={{$parameter["topic"]}}',
		description: 'Emit events to notif.sh',
		defaults: {
			name: 'Notif',
		},
		inputs: ['main'],
		outputs: ['main'],
		credentials: [
			{
				name: 'notifApi',
				required: true,
			},
		],
		properties: [
			{
				displayName: 'Topic',
				name: 'topic',
				type: 'string',
				default: '',
				required: true,
				placeholder: 'orders.new',
				description: 'The topic to emit the event to',
			},
			{
				displayName: 'Data',
				name: 'data',
				type: 'json',
				default: '{}',
				required: true,
				description: 'The event data as JSON',
			},
		],
	};

	async execute(this: IExecuteFunctions): Promise<INodeExecutionData[][]> {
		const items = this.getInputData();
		const returnData: INodeExecutionData[] = [];

		const credentials = await this.getCredentials('notifApi');
		const serverUrl = credentials.serverUrl as string;

		for (let i = 0; i < items.length; i++) {
			try {
				const topic = this.getNodeParameter('topic', i) as string;
				const dataParam = this.getNodeParameter('data', i) as string;

				let data: Record<string, unknown>;
				try {
					data = typeof dataParam === 'string' ? JSON.parse(dataParam) : dataParam;
				} catch {
					throw new Error('Invalid JSON in Data field');
				}

				const response = await this.helpers.request({
					method: 'POST',
					url: `${serverUrl}/api/v1/emit`,
					headers: {
						Authorization: `Bearer ${credentials.apiKey}`,
						'Content-Type': 'application/json',
					},
					body: {
						topic,
						data,
					},
					json: true,
				});

				returnData.push({
					json: response,
					pairedItem: { item: i },
				});
			} catch (error) {
				if (this.continueOnFail()) {
					returnData.push({
						json: { error: (error as Error).message },
						pairedItem: { item: i },
					});
					continue;
				}
				throw error;
			}
		}

		return [returnData];
	}
}
