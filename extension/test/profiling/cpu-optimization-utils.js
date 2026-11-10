/**
 * CPU Optimization Utilities
 * Phase 4 Task 10: Debouncing, throttling, and indexed filtering
 */

/**
 * Debounce function - waits for delay after last call
 */
function debounce(func, delay = 300) {
  let timeoutId = null;
  
  return function debounced(...args) {
    const context = this;
    
    if (timeoutId) {
      clearTimeout(timeoutId);
    }
    
    timeoutId = setTimeout(() => {
      func.apply(context, args);
      timeoutId = null;
    }, delay);
  };
}

/**
 * Throttle function - limits execution to once per interval
 */
function throttle(func, interval = 50) {
  let lastCall = 0;
  let timeoutId = null;
  
  return function throttled(...args) {
    const context = this;
    const now = Date.now();
    const timeSinceLastCall = now - lastCall;
    
    if (timeSinceLastCall >= interval) {
      lastCall = now;
      func.apply(context, args);
    } else {
      // Schedule for next available slot
      if (timeoutId) {
        clearTimeout(timeoutId);
      }
      
      timeoutId = setTimeout(() => {
        lastCall = Date.now();
        func.apply(context, args);
        timeoutId = null;
      }, interval - timeSinceLastCall);
    }
  };
}

/**
 * Request Animation Frame batcher
 * Batches multiple updates into single RAF
 */
class RAFBatcher {
  constructor() {
    this.pending = [];
    this.rafId = null;
  }
  
  schedule(callback) {
    this.pending.push(callback);
    
    if (!this.rafId) {
      this.rafId = requestAnimationFrame(() => this.flush());
    }
  }
  
  flush() {
    const callbacks = this.pending;
    this.pending = [];
    this.rafId = null;
    
    for (const callback of callbacks) {
      try {
        callback();
      } catch (error) {
        console.error('RAF batch callback error:', error);
      }
    }
  }
  
  clear() {
    if (this.rafId) {
      cancelAnimationFrame(this.rafId);
      this.rafId = null;
    }
    this.pending = [];
  }
}

/**
 * Indexed filter for O(1) lookups
 */
class IndexedFilter {
  constructor(items, indexKeys = ['priority', 'status']) {
    this.items = items;
    this.indexes = {};
    this.indexKeys = indexKeys;
    this.buildIndexes();
  }
  
  buildIndexes() {
    // Build index for each key
    for (const key of this.indexKeys) {
      this.indexes[key] = new Map();
      
      for (let i = 0; i < this.items.length; i++) {
        const item = this.items[i];
        const value = item[key];
        
        if (!this.indexes[key].has(value)) {
          this.indexes[key].set(value, []);
        }
        
        this.indexes[key].get(value).push(i);
      }
    }
  }
  
  filter(predicate) {
    const results = [];
    
    // Try to use index if predicate is simple equality check
    const indexResult = this.tryIndexLookup(predicate);
    if (indexResult !== null) {
      return indexResult;
    }
    
    // Fall back to linear scan
    for (const item of this.items) {
      if (predicate(item)) {
        results.push(item);
      }
    }
    
    return results;
  }
  
  tryIndexLookup(predicate) {
    // Detect simple predicates like (item => item.priority === 'P0')
    const predicateStr = predicate.toString();
    
    for (const key of this.indexKeys) {
      const pattern = new RegExp(`\\.${key}\\s*===\\s*['"]([^'"]+)['"]`);
      const match = predicateStr.match(pattern);
      
      if (match) {
        const value = match[1];
        const indices = this.indexes[key].get(value) || [];
        return indices.map(i => this.items[i]);
      }
    }
    
    return null;
  }
  
  getIndex(key) {
    return this.indexes[key];
  }
}

/**
 * Partial sorter - sorts only top N results
 */
function partialSort(array, compareFn, topN) {
  if (array.length <= topN) {
    return array.sort(compareFn);
  }
  
  // Use partial quicksort / heap approach
  // For simplicity, use native sort but return only top N
  const sorted = [...array].sort(compareFn);
  return sorted.slice(0, topN);
}

/**
 * Incremental search - streams results as they're found
 */
class IncrementalSearch {
  constructor(items, searchFn, batchSize = 100) {
    this.items = items;
    this.searchFn = searchFn;
    this.batchSize = batchSize;
    this.currentIndex = 0;
    this.results = [];
    this.isComplete = false;
  }
  
  next() {
    if (this.isComplete) {
      return { done: true, results: this.results };
    }
    
    const endIndex = Math.min(this.currentIndex + this.batchSize, this.items.length);
    
    for (let i = this.currentIndex; i < endIndex; i++) {
      const item = this.items[i];
      if (this.searchFn(item)) {
        this.results.push(item);
      }
    }
    
    this.currentIndex = endIndex;
    
    if (this.currentIndex >= this.items.length) {
      this.isComplete = true;
      return { done: true, results: this.results };
    }
    
    return { done: false, results: this.results, progress: this.currentIndex / this.items.length };
  }
  
  hasMore() {
    return !this.isComplete;
  }
}

module.exports = {
  debounce,
  throttle,
  RAFBatcher,
  IndexedFilter,
  partialSort,
  IncrementalSearch
};