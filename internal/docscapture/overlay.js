// Overlay injected by annotate.go via chromedp.Evaluate. It draws an arrow +
// label (or a box) for each annotation spec, anchored to the target element's
// live geometry (getBoundingClientRect), and returns the selectors that matched
// no element so the Go side can fail loud. The spec array is spliced in as a
// JSON literal in place of the SPECS placeholder (json.Marshal → injection-safe).
(function () {
  var specs = __SPECS__;
  var missing = [];
  var GOLD = '#d99a3b';
  function resolve(sel) {
    if (sel.indexOf('@button:') === 0) {
      var label = sel.slice(8);
      var btns = Array.prototype.slice.call(
        document.querySelectorAll('button, a, [role=button]')
      );
      return btns.find(function (b) {
        return (b.textContent || '').trim() === label;
      }) || null;
    }
    return document.querySelector(sel);
  }
  var layer = document.createElement('div');
  layer.style.cssText =
    'position:absolute;left:0;top:0;width:100%;height:100%;' +
    'pointer-events:none;z-index:2147483647;';
  document.body.appendChild(layer);
  specs.forEach(function (s) {
    var el = resolve(s.selector);
    if (!el) { missing.push(s.selector); return; }
    var r = el.getBoundingClientRect();
    if (r.width === 0 || r.height === 0) { missing.push(s.selector); return; }
    var x = r.left + window.scrollX, y = r.top + window.scrollY;
    if (s.box) {
      var box = document.createElement('div');
      box.style.cssText =
        'position:absolute;left:' + (x - 4) + 'px;top:' + (y - 4) + 'px;' +
        'width:' + (r.width + 8) + 'px;height:' + (r.height + 8) + 'px;' +
        'border:3px solid ' + GOLD + ';border-radius:6px;';
      layer.appendChild(box);
    }
    if (s.text) {
      var side = s.side || 'right';
      var ly = y + r.height / 2;
      var lab = document.createElement('div');
      lab.textContent = s.text;
      var lx = side === 'left' ? x - 8 : x + r.width + 8;
      var align = side === 'left'
        ? 'right:' + (document.documentElement.clientWidth - lx) + 'px;'
        : 'left:' + lx + 'px;';
      lab.style.cssText =
        'position:absolute;' + align + 'top:' + (ly - 12) + 'px;' +
        'background:' + GOLD + ';color:#1b1b1b;' +
        'font:600 13px system-ui,sans-serif;padding:2px 8px;' +
        'border-radius:4px;white-space:nowrap;';
      layer.appendChild(lab);
      var arrow = document.createElement('div');
      var ax = side === 'left' ? x - 6 : x + r.width;
      arrow.style.cssText =
        'position:absolute;left:' + ax + 'px;top:' + (ly - 1) + 'px;' +
        'width:8px;height:3px;background:' + GOLD + ';';
      layer.appendChild(arrow);
    }
  });
  return missing;
})();
