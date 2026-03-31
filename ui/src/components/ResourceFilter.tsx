import { useState, useCallback, useEffect } from 'react';
import type { FilterState } from '../hooks/useFilterState';

interface ResourceFilterProps {
  onFilterChange: (filters: FilterState) => void;
  filters: FilterState;
  placeholder?: string;
}

function ResourceFilter({ onFilterChange, filters, placeholder }: ResourceFilterProps) {
  const [name, setName] = useState(filters.name);
  const [labelSelector, setLabelSelector] = useState(filters.labelSelector);

  useEffect(() => {
    setName(filters.name);
    setLabelSelector(filters.labelSelector);
  }, [filters.name, filters.labelSelector]);

  const handleApply = useCallback(() => {
    onFilterChange({ name, labelSelector });
  }, [name, labelSelector, onFilterChange]);

  const handleClear = useCallback(() => {
    setName('');
    setLabelSelector('');
    onFilterChange({ name: '', labelSelector: '' });
  }, [onFilterChange]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') {
        handleApply();
      }
    },
    [handleApply]
  );

  const hasFilters = name || labelSelector;

  return (
    <div className="flex items-center gap-2 flex-wrap">
      <div className="relative flex-1 min-w-[200px]">
        <svg className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-stone-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        <input
          type="text"
          value={name}
          onChange={(e) => setName(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder || 'Filter by name...'}
          className="block w-full pl-9 pr-3 py-2 rounded-lg border border-stone-200 bg-white shadow-sm focus:border-primary-500 focus:ring-primary-500 text-sm text-stone-700 placeholder:text-stone-400"
        />
      </div>

      <div className="relative min-w-[200px] sm:min-w-[240px]">
        <svg className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-stone-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <path d="M20.59 13.41l-7.17 7.17a2 2 0 01-2.83 0L2 12V2h10l8.59 8.59a2 2 0 010 2.82z" />
          <line x1="7" y1="7" x2="7.01" y2="7" />
        </svg>
        <input
          type="text"
          value={labelSelector}
          onChange={(e) => setLabelSelector(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Label selector (e.g. app=myapp)"
          className="block w-full pl-9 pr-3 py-2 rounded-lg border border-stone-200 bg-white shadow-sm focus:border-primary-500 focus:ring-primary-500 text-sm text-stone-700 placeholder:text-stone-400"
        />
      </div>

      <button
        onClick={handleApply}
        className="px-4 py-2 text-sm font-medium text-white bg-stone-900 rounded-lg hover:bg-stone-800 transition-colors shadow-sm"
      >
        Filter
      </button>

      {hasFilters && (
        <button
          onClick={handleClear}
          className="px-3 py-2 text-sm text-stone-500 hover:text-stone-700 transition-colors font-medium"
        >
          Clear
        </button>
      )}
    </div>
  );
}

export default ResourceFilter;
