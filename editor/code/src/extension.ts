// import * as path from 'path';
import { commands, workspace, ExtensionContext } from 'vscode';

import {
	Executable,
	LanguageClient,
	LanguageClientOptions,
	ServerOptions
} from 'vscode-languageclient/node';

let client: LanguageClient;

async function startClient(binaryPath: string): Promise<void> {

	const executable: Executable = {
		command: binaryPath,
		args: [],
		options: {env: process.env},
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
	let cfg = workspace.getConfiguration('jsonnet.lsp');

	const binaryPath: string = cfg.get('binaryPath') ?? 
		// use bundled language server if one is not provided
		context.asAbsolutePath(`jsonnet-lsp_${process.platform}_${process.arch}`);

	await startClient(binaryPath);

	context.subscriptions.push(
		commands.registerCommand('jsonnet.lsp.restart', async function (): Promise<void> {
			if (client.isRunning()) {
				await client.stop();
				client.outputChannel.dispose();
			}

			await startClient(binaryPath);
		}),
	);
}

export function deactivate(): Thenable<void> | undefined {
	if (!client) {
		return undefined;
	}
	return client.stop();
}