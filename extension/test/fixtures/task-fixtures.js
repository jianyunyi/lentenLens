/**
 * Task Fixtures - Generate deterministic test task data
 */

/**
 * Simple seeded random number generator for reproducibility
 * @param {number} seed - Seed value
 * @returns {() => number} Function that returns pseudo-random numbers [0, 1)
 */
function createSeededRandom(seed) {
  let value = seed;
  return () => {
    value = (value * 9301 + 49297) % 233280;
    return value / 233280;
  };
}

/**
 * Generate deterministic task objects
 * @param {number} count - Number of tasks to generate
 * @param {number} [seed=42] - Seed for reproducibility
 * @returns {Object[]} Array of task objects
 */
function generateTasks(count, seed = 42) {
  const random = createSeededRandom(seed);
  const priorities = ['P0', 'P1', 'P2', 'P3'];
  const statuses = ['pending', 'in_progress', 'blocked', 'completed'];
  const tasks = [];

  for (let i = 0; i < count; i++) {
    const priorityIndex = Math.floor(random() * priorities.length);
    const statusIndex = Math.floor(random() * statuses.length);
    const depCount = Math.floor(random() * 3); // 0-2 dependencies

    const dependencies = [];
    for (let d = 0; d < depCount; d++) {
      const depId = Math.floor(random() * Math.max(1, i));
      if (depId !== i && !dependencies.includes(depId)) {
        dependencies.push(depId);
      }
    }

    tasks.push({
      id: `task-${i}`,
      name: `Task ${i}`,
      priority: priorities[priorityIndex],
      status: statuses[statusIndex],
      progress: Math.floor(random() * 101), // 0-100
      dependencies: dependencies
    });
  }

  return tasks;
}

module.exports = {
  generateTasks,
  createSeededRandom
};
