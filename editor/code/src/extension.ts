// import * as path from 'path';
import { commands, workspace, ExtensionContext } from 'vscode';

import {
	Executable,
	LanguageClient,
	LanguageClientOptions,
	ServerOptions
} from 'vscode-languageclient/node';

let client: LanguageClient;

async function startClient(): Promise<void> {
	let cfg = workspace.getConfiguration('jsonnet.lsp');

	const executable: Executable = {
		command: cfg.get('serverBinary') ?? "jsonnet-lsp",
		args: [],
		options: {
			env: process.env,
		},
	};

	const serverOptions: ServerOptions = {
		run: executable,
		debug: executable,

	};

	const clientOptions: LanguageClientOptions = {
		documentSelector: [{ scheme: 'file', language: 'jsonnet' }],
	};

	client = new LanguageClient(
		'JsonnetLSP',
		'Jsonnet Language Server',
		serverOptions,
		clientOptions
	);

	client.start();
}

export async function activate(context: ExtensionContext) {

	await startClient();

	context.subscriptions.push(
		commands.registerCommand('jsonnet.lsp.restart', async function (): Promise<void> {
			if (client.isRunning()) {
				await client.stop();
				client.outputChannel.dispose();
			}

			await startClient();
		}),
	);
}

export function deactivate(): Thenable<void> | undefined {
	if (!client) {
		return undefined;
	}
	return client.stop();
}