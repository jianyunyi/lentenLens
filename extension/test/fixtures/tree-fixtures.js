/**
 * Tree Fixtures - Generate deterministic call tree structures
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
 * Generate a call tree with configurable depth and branching
 * @param {number} depth - Maximum tree depth
 * @param {number} [branchingFactor=2] - Average children per node
 * @param {number} [seed=42] - Seed for reproducibility
 * @returns {Object} Root node of the tree
 */
function generateCallTree(depth, branchingFactor = 2, seed = 42) {
  const random = createSeededRandom(seed);
  let nodeId = 0;

  function buildNode(currentDepth) {
    const id = nodeId++;
    const numChildren = currentDepth < depth 
      ? Math.floor(random() * (branchingFactor + 1))
      : 0;

    const node = {
      id: `node-${id}`,
      name: `Function ${id}`,
      line: Math.floor(random() * 1000) + 1,
      children: []
    };

    for (let i = 0; i < numChildren; i++) {
      node.children.push(buildNode(currentDepth + 1));
    }

    return node;
  }

  return buildNode(0);
}

/**
 * Calculate statistics about a tree
 * @param {Object} node - Root node
 * @returns {Object} Statistics object
 */
function getTreeStats(node) {
  let nodeCount = 0;
  let maxDepth = 0;
  let totalChildren = 0;

  function traverse(n, depth) {
    nodeCount++;
    maxDepth = Math.max(maxDepth, depth);
    totalChildren += n.children.length;
    
    for (const child of n.children) {
      traverse(child, depth + 1);
    }
  }

  traverse(node, 0);

  return {
    nodeCount,
    maxDepth,
    avgChildren: totalChildren / Math.max(1, nodeCount)
  };
}

module.exports = {
  generateCallTree,
  getTreeStats,
  createSeededRandom
};
