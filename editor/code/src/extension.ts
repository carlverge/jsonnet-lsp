import { commands, workspace, ExtensionContext, window, EventEmitter, TextDocumentContentProvider, Uri, ViewColumn, WorkspaceConfiguration } from 'vscode';

import {
	DidChangeConfigurationNotification,
	Executable,
	ExecuteCommandRequest,
	LanguageClient,
	LanguageClientOptions,
	ServerOptions,
} from 'vscode-languageclient/node';

let client: LanguageClient;


// previewProvider is a virtual content provider which displays ephemeral preview output
// for jsonnet evaluation results. There is one preview pane per workspace, and it will
// update on every evaluation.
const previewProvider = new class implements TextDocumentContentProvider {
	data: string | undefined;
	uriScheme = 'jsonnetpreview';
	previewPaneURI = Uri.parse(`${this.uriScheme}:preview.json`);
	onDidChangeEmitter = new EventEmitter<Uri>();
	onDidChange = this.onDidChangeEmitter.event;

	previewDidChange(data: string) {
		this.data = data;
		this.onDidChangeEmitter.fire(this.previewPaneURI);
	}

	provideTextDocumentContent(uri: Uri): string {
		return this.data ?? "No jsonnet evaluation results";
	}
};


async function startClient(binaryPath: string, cfg: WorkspaceConfiguration): Promise<void> {

	const executable: Executable = {
		command: binaryPath,
		args: ["lsp"],
		options: { env: process.env },
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
	client.sendNotification(DidChangeConfigurationNotification.type, {settings: cfg});
}

function builtinBinaryPath(): string {
	const ext = process.platform === 'win32' ? '.exe' : '';
	return `jsonnet-lsp_${process.platform}_${process.arch}${ext}`;
}


type EvaluateResult = {
	output: string;
};

export async function activate(context: ExtensionContext) {
	let cfg = workspace.getConfiguration('jsonnet.lsp');

	// use bundled language server if one is not provided
	const defaultBinary: string = context.asAbsolutePath(builtinBinaryPath());
	const binaryPath: string = cfg.get('binaryPath') ? cfg.get('binaryPath') ?? defaultBinary : defaultBinary;

	await startClient(binaryPath, cfg);

	context.subscriptions.push(
		commands.registerCommand('jsonnet.lsp.restart', async function (): Promise<void> {
			if (client.isRunning()) {
				await client.stop();
				client.outputChannel.dispose();
			}

			await startClient(binaryPath, cfg);
		}),
		workspace.registerTextDocumentContentProvider(previewProvider.uriScheme, previewProvider),
		commands.registerCommand('jsonnet.lsp.evaluate', async function (): Promise<void> {
			const editor = window.activeTextEditor;
			if (editor === undefined) {
				window.showErrorMessage("jsonnet: cannot evaluate file, no active editor");
				return;
			}

			// do nothing if it's not a jsonnet file
			if (editor.document.languageId !== "jsonnet") {
				return;
			}

			if (!client.isRunning()) {
				window.showErrorMessage("jsonnet: cannot evaluate file, language server not running");
				return;
			}

			const result: EvaluateResult = await client.sendRequest(ExecuteCommandRequest.type, {
				command: "jsonnet.lsp.evaluate",
				arguments: [JSON.stringify({
					textDocument: { uri: editor.document.uri.toString() }
				})]
			}).catch(err => window.showErrorMessage(`jsonnet: failed to evaluate file ${err}`));

			previewProvider.previewDidChange(result.output);
			
			const doc = { ...(await workspace.openTextDocument(previewProvider.previewPaneURI)), languageId: "json" };
			await window.showTextDocument(doc, ViewColumn.Beside, true);
		})
	);
}

export function deactivate(): Thenable<void> | undefined {
	if (!client) {
		return undefined;
	}
	return client.stop();
}