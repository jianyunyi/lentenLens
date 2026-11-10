/**
 * Object Pool for Task Reuse
 * Phase 4 Task 9: Memory optimization through object pooling
 */

class ObjectPool {
  constructor(factory, reset, maxSize = 1000) {
    this.factory = factory;
    this.reset = reset;
    this.maxSize = maxSize;
    this.pool = [];
    this.inUse = new Set();
    this.stats = {
      created: 0,
      reused: 0,
      released: 0,
      currentSize: 0
    };
  }

  /**
   * Acquire object from pool or create new one
   */
  acquire() {
    let obj;
    
    if (this.pool.length > 0) {
      obj = this.pool.pop();
      this.stats.reused++;
    } else {
      obj = this.factory();
      this.stats.created++;
    }
    
    this.inUse.add(obj);
    this.stats.currentSize = this.pool.length;
    return obj;
  }

  /**
   * Release object back to pool
   */
  release(obj) {
    if (!this.inUse.has(obj)) {
      return; // Already released or not from this pool
    }
    
    this.inUse.delete(obj);
    
    // Reset object state
    if (this.reset) {
      this.reset(obj);
    }
    
    // Add to pool if not at max capacity
    if (this.pool.length < this.maxSize) {
      this.pool.push(obj);
      this.stats.released++;
    }
    
    this.stats.currentSize = this.pool.length;
  }

  /**
   * Release all in-use objects
   */
  releaseAll() {
    const toRelease = Array.from(this.inUse);
    for (const obj of toRelease) {
      this.release(obj);
    }
  }

  /**
   * Clear pool
   */
  clear() {
    this.pool = [];
    this.inUse.clear();
    this.stats.currentSize = 0;
  }

  /**
   * Get pool statistics
   */
  getStats() {
    return {
      ...this.stats,
      inUse: this.inUse.size,
      available: this.pool.length,
      reuseRate: this.stats.created > 0 
        ? (this.stats.reused / (this.stats.created + this.stats.reused) * 100).toFixed(2) + '%'
        : '0%'
    };
  }
}

/**
 * Task object factory
 */
function createTaskFactory() {
  return () => ({
    id: null,
    name: null,
    priority: null,
    status: null,
    progress: 0,
    dependencies: [],
    codeRange: null,
    description: null,
    executionTime: null,
    memoryUsage: null
  });
}

/**
 * Task object reset function
 */
function resetTask(task) {
  task.id = null;
  task.name = null;
  task.priority = null;
  task.status = null;
  task.progress = 0;
  task.dependencies = [];
  task.codeRange = null;
  task.description = null;
  task.executionTime = null;
  task.memoryUsage = null;
}

/**
 * Copy properties from source to pooled task
 */
function populateTask(pooledTask, sourceTask) {
  pooledTask.id = sourceTask.id;
  pooledTask.name = sourceTask.name;
  pooledTask.priority = sourceTask.priority;
  pooledTask.status = sourceTask.status;
  pooledTask.progress = sourceTask.progress;
  pooledTask.dependencies = sourceTask.dependencies ? [...sourceTask.dependencies] : [];
  pooledTask.codeRange = sourceTask.codeRange ? { ...sourceTask.codeRange } : null;
  pooledTask.description = sourceTask.description;
  pooledTask.executionTime = sourceTask.executionTime;
  pooledTask.memoryUsage = sourceTask.memoryUsage;
  return pooledTask;
}

/**
 * Create global task pool singleton
 */
let globalTaskPool = null;

function getTaskPool() {
  if (!globalTaskPool) {
    globalTaskPool = new ObjectPool(createTaskFactory(), resetTask, 1000);
  }
  return globalTaskPool;
}

module.exports = {
  ObjectPool,
  createTaskFactory,
  resetTask,
  populateTask,
  getTaskPool
};