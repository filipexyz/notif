import type {
	IAuthenticateGeneric,
	ICredentialTestRequest,
	ICredentialType,
	INodeProperties,
} from 'n8n-workflow';

export class NotifApi implements ICredentialType {
	name = 'notifApi';
	displayName = 'Notif API';
	documentationUrl = 'https://docs.notif.sh';
	properties: INodeProperties[] = [
		{
			displayName: 'API Key',
			name: 'apiKey',
			type: 'string',
			typeOptions: { password: true },
			default: '',
			required: true,
			description: 'Your notif.sh API key (starts with nsh_)',
		},
		{
			displayName: 'Server URL',
			name: 'serverUrl',
			type: 'string',
			default: 'https://api.notif.sh',
			description: 'The notif.sh API server URL',
		},
	];

	authenticate: IAuthenticateGeneric = {
		type: 'generic',
		properties: {
			headers: {
				Authorization: '=Bearer {{$credentials.apiKey}}',
			},
		},
	};

	test: ICredentialTestRequest = {
		request: {
			baseURL: '={{$credentials.serverUrl}}',
			url: '/health',
			method: 'GET',
		},
	};
}
