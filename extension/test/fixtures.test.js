const test = require('node:test');
const assert = require('node:assert/strict');
const { generateTasks } = require('./fixtures/task-fixtures');
const { generateCallTree, getTreeStats } = require('./fixtures/tree-fixtures');
const {
  createMockTextEditor,
  createMockExtensionContext,
  createMockMemento,
  createMockSecrets,
  createMockEnvironmentVariableCollection,
  createMockCommandsAPI
} = require('./fixtures/api-mocks');

// Task Fixtures Tests
test('generateTasks creates correct number of tasks', t => {
  const tasks = generateTasks(100);
  assert.equal(tasks.length, 100);
});

test('generateTasks generates deterministic output with seed', t => {
  const tasks1 = generateTasks(50, 42);
  const tasks2 = generateTasks(50, 42);
  
  assert.deepEqual(tasks1, tasks2, 'Same seed should produce identical tasks');
});

test('generateTasks produces different output with different seed', t => {
  const tasks1 = generateTasks(50, 42);
  const tasks2 = generateTasks(50, 100);
  
  assert.notDeepEqual(tasks1, tasks2, 'Different seeds should produce different tasks');
});

test('generateTasks produces valid task objects', t => {
  const tasks = generateTasks(10);
  
  for (const task of tasks) {
    assert.equal(typeof task.id, 'string');
    assert.ok(task.id.startsWith('task-'));
    assert.equal(typeof task.name, 'string');
    assert.ok(['P0', 'P1', 'P2', 'P3'].includes(task.priority));
    assert.ok(['pending', 'in_progress', 'blocked', 'completed'].includes(task.status));
    assert.ok(typeof task.progress === 'number');
    assert.ok(task.progress >= 0 && task.progress <= 100);
    assert.ok(Array.isArray(task.dependencies));
  }
});

test('generateTasks handles large counts', t => {
  const tasks = generateTasks(1000);
  assert.equal(tasks.length, 1000);
  assert.ok(tasks[0].id);
  assert.ok(tasks[999].id);
});

test('generateTasks dependencies are valid', t => {
  const tasks = generateTasks(100);
  
  for (let i = 0; i < tasks.length; i++) {
    const task = tasks[i];
    for (const depId of task.dependencies) {
      // Dependency should refer to a task index less than current
      assert.ok(depId < i, `Task ${i} has invalid dependency ${depId}`);
    }
  }
});

// Tree Fixtures Tests
test('generateCallTree creates tree structure', t => {
  const tree = generateCallTree(3, 2);
  
  assert.ok(tree.id);
  assert.ok(tree.name);
  assert.ok(typeof tree.line === 'number');
  assert.ok(Array.isArray(tree.children));
});

test('generateCallTree respects depth parameter', t => {
  const tree = generateCallTree(2, 2, 42);
  const stats = getTreeStats(tree);
  
  assert.ok(stats.maxDepth <= 2);
});

test('generateCallTree is deterministic with seed', t => {
  const tree1 = generateCallTree(3, 2, 42);
  const tree2 = generateCallTree(3, 2, 42);
  
  assert.deepEqual(tree1, tree2, 'Same seed should produce identical trees');
});

test('generateCallTree produces different output with different seed', t => {
  const tree1 = generateCallTree(3, 2, 42);
  const tree2 = generateCallTree(3, 2, 100);
  
  assert.notDeepEqual(tree1, tree2, 'Different seeds should produce different trees');
});

test('getTreeStats calculates correct statistics', t => {
  const tree = generateCallTree(5, 3, 42);
  const stats = getTreeStats(tree);
  
  assert.ok(typeof stats.nodeCount === 'number');
  assert.ok(stats.nodeCount >= 1);
  assert.ok(typeof stats.maxDepth === 'number');
  assert.ok(stats.maxDepth >= 0);
  assert.ok(typeof stats.avgChildren === 'number');
  assert.ok(stats.avgChildren >= 0);
});

test('getTreeStats handles single node tree', t => {
  const tree = generateCallTree(0, 2, 42); // 0 depth = no children
  const stats = getTreeStats(tree);
  
  assert.equal(stats.nodeCount, 1);
  assert.equal(stats.maxDepth, 0);
  assert.equal(stats.avgChildren, 0);
});

// API Mocks Tests
test('createMockTextEditor returns valid editor object', t => {
  const editor = createMockTextEditor();
  
  assert.ok(editor.document);
  assert.ok(editor.selection);
  assert.ok(editor.visibleRanges);
  assert.equal(editor.document.languageId, 'go');
  assert.ok(Array.isArray(editor.visibleRanges));
});

test('createMockExtensionContext returns valid context', t => {
  const context = createMockExtensionContext();
  
  assert.ok(Array.isArray(context.subscriptions));
  assert.ok(context.workspaceState);
  assert.ok(context.globalState);
  assert.ok(context.secrets);
  assert.ok(context.extensionUri);
  assert.ok(context.storagePath);
});

test('createMockMemento stores and retrieves values', async t => {
  const memento = createMockMemento();
  
  await memento.update('key1', 'value1');
  assert.equal(memento.get('key1'), 'value1');
  assert.equal(memento.get('nonexistent', 'default'), 'default');
});

test('createMockMemento deletes values', async t => {
  const memento = createMockMemento();
  
  await memento.update('key1', 'value1');
  assert.equal(memento.get('key1'), 'value1');
  
  await memento.update('key1', undefined);
  assert.equal(memento.get('key1', 'default'), 'default');
});

test('createMockMemento lists keys', async t => {
  const memento = createMockMemento();
  
  await memento.update('key1', 'value1');
  await memento.update('key2', 'value2');
  
  const keys = memento.keys();
  assert.equal(keys.length, 2);
  assert.ok(keys.includes('key1'));
  assert.ok(keys.includes('key2'));
});

test('createMockSecrets stores and retrieves secrets', async t => {
  const secrets = createMockSecrets();
  
  await secrets.store('password', 'secret123');
  const retrieved = await secrets.get('password');
  assert.equal(retrieved, 'secret123');
});

test('createMockSecrets returns null for nonexistent secret', async t => {
  const secrets = createMockSecrets();
  
  const retrieved = await secrets.get('nonexistent');
  assert.equal(retrieved, null);
});

test('createMockSecrets deletes secrets', async t => {
  const secrets = createMockSecrets();
  
  await secrets.store('password', 'secret123');
  await secrets.delete('password');
  const retrieved = await secrets.get('password');
  assert.equal(retrieved, null);
});

test('createMockEnvironmentVariableCollection manages variables', t => {
  const env = createMockEnvironmentVariableCollection();
  
  env.replace('VAR', 'value');
  assert.ok(env.get('VAR'));
  assert.equal(env.get('VAR').operation, 'replace');
});

test('createMockCommandsAPI registers and executes commands', async t => {
  const commands = createMockCommandsAPI();
  
  let callCount = 0;
  commands.registerCommand('test-cmd', async (arg) => {
    callCount++;
    return `received: ${arg}`;
  });
  
  const result = await commands.executeCommand('test-cmd', 'hello');
  assert.equal(result, 'received: hello');
  assert.equal(callCount, 1);
});

test('createMockCommandsAPI throws for unregistered command', async t => {
  const commands = createMockCommandsAPI();
  
  await assert.rejects(
    () => commands.executeCommand('nonexistent'),
    /not found/
  );
});

test('createMockCommandsAPI lists registered commands', async t => {
  const commands = createMockCommandsAPI();
  
  commands.registerCommand('cmd1', async () => {});
  commands.registerCommand('cmd2', async () => {});
  
  const list = await commands.getCommands();
  assert.equal(list.length, 2);
  assert.ok(list.includes('cmd1'));
  assert.ok(list.includes('cmd2'));
});

test('createMockCommandsAPI disposes command registration', async t => {
  const commands = createMockCommandsAPI();
  
  const dispose = commands.registerCommand('cmd', async () => 'result');
  await assert.doesNotReject(() => commands.executeCommand('cmd'));
  
  dispose.dispose();
  await assert.rejects(() => commands.executeCommand('cmd'));
});
