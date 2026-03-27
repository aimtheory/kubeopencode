import React, { useState, useEffect, useCallback } from 'react';

interface WebUIPanelProps {
  namespace: string;
  agentName: string;
}

type PanelMode = 'collapsed' | 'expanded' | 'maximized';

function WebUIPanel({ namespace, agentName }: WebUIPanelProps) {
  const [mode, setMode] = useState<PanelMode>('collapsed');
  const [iframeLoaded, setIframeLoaded] = useState(false);

  const webUIUrl = `/api/v1/namespaces/${namespace}/agents/${agentName}/web/`;

  const handleLaunch = useCallback(() => {
    if (window.innerWidth < 1024) {
      window.open(webUIUrl, '_blank', 'noopener,noreferrer');
    } else {
      setMode('expanded');
    }
  }, [webUIUrl]);

  const handleOpenNewTab = useCallback(() => {
    window.open(webUIUrl, '_blank', 'noopener,noreferrer');
  }, [webUIUrl]);

  // ESC key to exit maximized mode
  useEffect(() => {
    if (mode !== 'maximized') return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation();
        setMode('expanded');
      }
    };
    document.addEventListener('keydown', handler, true);
    return () => document.removeEventListener('keydown', handler, true);
  }, [mode]);

  // Reset iframe loaded state when panel is closed
  useEffect(() => {
    if (mode === 'collapsed') {
      setIframeLoaded(false);
    }
  }, [mode]);

  // Collapsed: launch CTA card
  if (mode === 'collapsed') {
    return (
      <div>
        <h3 className="text-[11px] font-display font-medium text-stone-400 uppercase tracking-wider mb-3">Web UI</h3>
        <div
          className="group relative overflow-hidden rounded-xl border border-stone-200 bg-gradient-to-br from-stone-900 via-stone-900 to-stone-800 cursor-pointer transition-all duration-200 hover:border-emerald-600/40 hover:shadow-lg hover:shadow-emerald-900/10"
          onClick={handleLaunch}
          role="button"
          tabIndex={0}
          onKeyDown={(e) => e.key === 'Enter' && handleLaunch()}
        >
          {/* Subtle grid pattern overlay */}
          <div className="absolute inset-0 opacity-[0.03]" style={{
            backgroundImage: 'linear-gradient(rgba(255,255,255,.1) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,.1) 1px, transparent 1px)',
            backgroundSize: '20px 20px'
          }} />

          <div className="relative px-5 py-4 flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="w-9 h-9 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center group-hover:bg-emerald-500/20 transition-colors">
                <svg className="w-4.5 h-4.5 text-emerald-400" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <rect x="2" y="3" width="20" height="14" rx="2" />
                  <path d="M8 21h8M12 17v4" />
                </svg>
              </div>
              <div>
                <span className="text-sm font-medium text-stone-200 group-hover:text-white transition-colors">
                  Launch OpenCode Web UI
                </span>
                <p className="text-[11px] text-stone-500 mt-0.5">Interactive AI coding session in your browser</p>
              </div>
            </div>
            <div className="flex items-center gap-3">
              <button
                onClick={(e) => { e.stopPropagation(); handleOpenNewTab(); }}
                className="text-[11px] text-stone-500 hover:text-stone-300 transition-colors px-2 py-1 rounded hover:bg-white/5"
                title="Open in new tab"
              >
                New tab
              </button>
              <svg className="w-4 h-4 text-stone-600 group-hover:text-emerald-400 group-hover:translate-x-0.5 transition-all" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M5 12h14M12 5l7 7-7 7" />
              </svg>
            </div>
          </div>
        </div>
      </div>
    );
  }

  // Expanded / Maximized
  const isMaximized = mode === 'maximized';

  return (
    <>
      {/* Maximized backdrop */}
      {isMaximized && (
        <div className="fixed inset-0 z-40 bg-black/60 backdrop-blur-sm" style={{ animation: 'fade-in 0.15s ease-out' }} />
      )}

      <div
        className={
          isMaximized
            ? 'fixed inset-3 z-50 bg-stone-950 flex flex-col rounded-2xl overflow-hidden shadow-2xl ring-1 ring-white/10'
            : 'bg-stone-950 rounded-xl overflow-hidden border border-stone-800 animate-fade-in'
        }
        style={isMaximized ? { animation: 'panel-maximize 0.2s cubic-bezier(0.16, 1, 0.3, 1)' } : undefined}
      >
        {/* Header bar */}
        <div className="px-4 py-2 bg-stone-900/80 flex items-center justify-between flex-shrink-0 border-b border-stone-800/60">
          <div className="flex items-center space-x-2.5">
            <span className={`inline-block w-1.5 h-1.5 rounded-full ${iframeLoaded ? 'bg-emerald-400' : 'bg-stone-600 animate-pulse'}`} />
            <span className="text-[11px] font-display font-medium text-stone-500 uppercase tracking-wider">OpenCode</span>
            <span className="text-[11px] text-stone-600 font-mono">{agentName}</span>
          </div>
          <div className="flex items-center">
            {/* Open in new tab */}
            <button
              onClick={handleOpenNewTab}
              className="p-1.5 rounded-md text-stone-500 hover:text-stone-300 hover:bg-white/5 transition-colors"
              title="Open in new tab"
            >
              <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M18 13v6a2 2 0 01-2 2H5a2 2 0 01-2-2V8a2 2 0 012-2h6" />
                <polyline points="15 3 21 3 21 9" />
                <line x1="10" y1="14" x2="21" y2="3" />
              </svg>
            </button>
            {/* Maximize / Restore */}
            <button
              onClick={() => setMode(isMaximized ? 'expanded' : 'maximized')}
              className="p-1.5 rounded-md text-stone-500 hover:text-stone-300 hover:bg-white/5 transition-colors"
              title={isMaximized ? 'Restore (Esc)' : 'Maximize'}
            >
              {isMaximized ? (
                <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <polyline points="4 14 10 14 10 20" />
                  <polyline points="20 10 14 10 14 4" />
                  <line x1="14" y1="10" x2="21" y2="3" />
                  <line x1="3" y1="21" x2="10" y2="14" />
                </svg>
              ) : (
                <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <polyline points="15 3 21 3 21 9" />
                  <polyline points="9 21 3 21 3 15" />
                  <line x1="21" y1="3" x2="14" y2="10" />
                  <line x1="3" y1="21" x2="10" y2="14" />
                </svg>
              )}
            </button>
            {/* Separator + Close */}
            <div className="w-px h-4 bg-stone-700/50 mx-1.5" />
            <button
              onClick={() => setMode('collapsed')}
              className="p-1.5 rounded-md text-stone-500 hover:text-red-400 hover:bg-red-500/10 transition-colors"
              title="Close"
            >
              <svg className="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </button>
          </div>
        </div>

        {/* iframe container */}
        <div className={isMaximized ? 'flex-1 relative min-h-0' : 'relative'}>
          {!iframeLoaded && (
            <div className="absolute inset-0 flex items-center justify-center bg-stone-950 z-10">
              <div className="flex flex-col items-center gap-3">
                <div className="w-5 h-5 border-2 border-stone-800 border-t-emerald-500 rounded-full animate-spin" />
                <span className="text-[11px] text-stone-600 font-mono">Connecting to OpenCode...</span>
              </div>
            </div>
          )}
          <iframe
            src={webUIUrl}
            className="w-full border-0 bg-stone-950 block"
            style={{ height: isMaximized ? '100%' : '70vh' }}
            sandbox="allow-same-origin allow-scripts allow-popups allow-forms allow-modals"
            allow="clipboard-read; clipboard-write"
            title={`OpenCode Web UI - ${agentName}`}
            onLoad={() => setIframeLoaded(true)}
          />
        </div>
      </div>
    </>
  );
}

export default WebUIPanel;
