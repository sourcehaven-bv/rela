/**
 * Document Watcher Module
 *
 * Handles live-reload for document preview pages.
 * Listens for SSE events and refreshes document content without full page reload.
 *
 * Usage:
 *   Add data-render-url attribute to document body element:
 *   <div id="document-body" data-render-url="/document/preview?entry=X&render=true">
 *
 * The module auto-initializes on DOMContentLoaded and registers with the app's
 * SSE refresh system via window._pageRefreshHandlers.
 */
(function() {
  'use strict';

  var DEBOUNCE_MS = 500;
  var debounceTimer = null;
  var indicator = null;
  var isUpdating = false;
  var renderURL = null;

  /**
   * Find and initialize the document watcher if on a document page.
   */
  function init() {
    var docBody = document.getElementById('document-body');
    if (!docBody || !docBody.dataset.renderUrl) {
      return; // Not a document page
    }

    renderURL = docBody.dataset.renderUrl;

    // Register refresh handler with app.js
    registerRefreshHandler();
  }

  /**
   * Register the document refresh handler with app.js.
   */
  function registerRefreshHandler() {
    if (!window._pageRefreshHandlers) {
      window._pageRefreshHandlers = [];
    }
    window._pageRefreshHandlers.push({
      id: 'document-watcher',
      handler: handleRefresh
    });

    // Cleanup on navigation
    document.addEventListener('htmx:beforeSwap', cleanup, { once: true });
  }

  /**
   * Clean up handlers when navigating away.
   */
  function cleanup(e) {
    if (e.detail.target && e.detail.target.id === 'content') {
      if (window._pageRefreshHandlers) {
        window._pageRefreshHandlers = window._pageRefreshHandlers.filter(function(h) {
          return h.id !== 'document-watcher';
        });
      }
    }
  }

  /**
   * Handle refresh event with debouncing.
   */
  function handleRefresh() {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(triggerDocumentRefresh, DEBOUNCE_MS);
  }

  /**
   * Create and show the update indicator.
   */
  function showIndicator() {
    if (!indicator) {
      indicator = document.createElement('div');
      indicator.className = 'doc-update-indicator';
      indicator.innerHTML = '<div class="mini-spinner"></div><span>Updating...</span>';
      document.body.appendChild(indicator);
    }
    // Force reflow before adding visible class
    indicator.offsetHeight; // eslint-disable-line no-unused-expressions
    indicator.classList.add('visible');
  }

  /**
   * Hide the update indicator.
   */
  function hideIndicator() {
    if (indicator) {
      indicator.classList.remove('visible');
    }
  }

  /**
   * Fetch new content and update the document.
   */
  function triggerDocumentRefresh() {
    if (isUpdating || !renderURL) return;
    isUpdating = true;
    showIndicator();

    // Save scroll position
    var scrollY = window.scrollY;
    var scrollX = window.scrollX;

    fetch(renderURL)
      .then(function(r) { return r.text(); })
      .then(function(html) {
        updateContent(html, scrollX, scrollY);
        hideIndicator();
        isUpdating = false;
      })
      .catch(function(err) {
        console.error('Document refresh failed:', err);
        hideIndicator();
        isUpdating = false;
      });
  }

  /**
   * Update the document content and restore scroll position.
   */
  function updateContent(html, scrollX, scrollY) {
    var docBody = document.getElementById('document-body');
    if (!docBody) return;

    docBody.outerHTML = html;

    // Re-initialize mermaid for any new diagrams
    if (typeof mermaid !== 'undefined') {
      var nodes = document.querySelectorAll('pre.mermaid:not([data-mermaid-processed])');
      if (nodes.length > 0) {
        nodes.forEach(function(n) { n.setAttribute('data-mermaid-processed', 'true'); });
        mermaid.run({ nodes: nodes });
      }
    }

    // Process any HTMX in the new content
    var newDocBody = document.getElementById('document-body');
    if (newDocBody && typeof htmx !== 'undefined') {
      htmx.process(newDocBody);
    }

    // Restore scroll position
    window.scrollTo(scrollX, scrollY);
  }

  // Initialize on DOM ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
