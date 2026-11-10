/**
 * API Mocks - Mock VS Code API objects for testing
 */

/**
 * Create a mock TextEditor object
 * @returns {Object} Mock TextEditor
 */
function createMockTextEditor() {
  return {
    document: {
      uri: { fsPath: '/mock/file.go' },
      languageId: 'go',
      isDirty: false,
      isClosed: false,
      lineCount: 100,
      getText: () => 'package main\n\nfunc main() {}',
      getWordRangeAtPosition: () => null,
      lineAt: (line) => ({
        text: `line ${line}`,
        range: { start: { line }, end: { line, character: 10 } }
      })
    },
    selection: {
      start: { line: 0, character: 0 },
      end: { line: 0, character: 10 },
      anchor: { line: 0, character: 0 },
      active: { line: 0, character: 10 },
      isEmpty: false,
      isSingleLine: true,
      isReversed: false
    },
    visibleRanges: [
      {
        start: { line: 0, character: 0 },
        end: { line: 50, character: 0 }
      }
    ],
    viewColumn: 1,
    options: {
      tabSize: 4,
      insertSpaces: true
    }
  };
}

/**
 * Create a mock ExtensionContext object
 * @returns {Object} Mock ExtensionContext
 */
function createMockExtensionContext() {
  const subscriptions = [];
  
  return {
    subscriptions,
    workspaceState: createMockMemento(),
    globalState: createMockMemento(),
    secrets: createMockSecrets(),
    extensionUri: { fsPath: '/mock/extension' },
    extensionPath: '/mock/extension',
    environmentVariableCollection: createMockEnvironmentVariableCollection(),
    storageUri: { fsPath: '/mock/storage' },
    storagePath: '/mock/storage',
    globalStorageUri: { fsPath: '/mock/global-storage' },
    globalStoragePath: '/mock/global-storage',
    logUri: { fsPath: '/mock/logs' },
    logPath: '/mock/logs',
    asAbsolutePath: (relativePath) => `/mock/extension/${relativePath}`,
    extension: {
      id: 'mock-extension',
      packageJSON: {},
      isActive: true,
      activate: async () => ({})
    }
  };
}

/**
 * Create a mock Memento object (for workspace/global state)
 * @returns {Object} Mock Memento
 */
function createMockMemento() {
  const storage = new Map();

  return {
    keys: () => Array.from(storage.keys()),
    get: (key, defaultValue) => storage.has(key) ? storage.get(key) : defaultValue,
    update: async (key, value) => {
      if (value === undefined) {
        storage.delete(key);
      } else {
        storage.set(key, value);
      }
    }
  };
}

/**
 * Create a mock Secrets object
 * @returns {Object} Mock Secrets
 */
function createMockSecrets() {
  const secrets = new Map();

  return {
    get: async (key) => secrets.get(key) || null,
    store: async (key, value) => secrets.set(key, value),
    delete: async (key) => secrets.delete(key),
    onDidChange: {
      _listeners: [],
      fire: function(key) {
        this._listeners.forEach(cb => cb({ key }));
      },
      subscribe: function(listener) {
        this._listeners.push(listener);
        return { dispose: () => {
          const idx = this._listeners.indexOf(listener);
          if (idx !== -1) this._listeners.splice(idx, 1);
        }};
      }
    }
  };
}

/**
 * Create a mock EnvironmentVariableCollection object
 * @returns {Object} Mock EnvironmentVariableCollection
 */
function createMockEnvironmentVariableCollection() {
  const vars = new Map();

  return {
    replace: (variable, value) => vars.set(variable, { value, operation: 'replace' }),
    append: (variable, value) => vars.set(variable, { value, operation: 'append' }),
    prepend: (variable, value) => vars.set(variable, { value, operation: 'prepend' }),
    get: (variable) => vars.get(variable),
    forEach: (callback) => vars.forEach((val, key) => callback(key, val)),
    delete: (variable) => vars.delete(variable),
    clear: () => vars.clear()
  };
}

/**
 * Create mock commands API
 * @returns {Object} Mock commands API
 */
function createMockCommandsAPI() {
  const commands = new Map();

  return {
    registerCommand: (commandId, callback) => {
      commands.set(commandId, callback);
      return { dispose: () => commands.delete(commandId) };
    },
    executeCommand: async (commandId, ...args) => {
      const command = commands.get(commandId);
      if (!command) {
        throw new Error(`Command '${commandId}' not found`);
      }
      return await command(...args);
    },
    getCommands: async () => Array.from(commands.keys())
  };
}

module.exports = {
  createMockTextEditor,
  createMockExtensionContext,
  createMockMemento,
  createMockSecrets,
  createMockEnvironmentVariableCollection,
  createMockCommandsAPI
};
