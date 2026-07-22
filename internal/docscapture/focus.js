// Computes the union bounding box of the annotation targets AND the drawn
// overlay (.rela-anno labels/arrows extend beyond their target), so
// clip="focus" crops to include the annotations. The SELS placeholder is the
// JSON array of target selectors; returns {X,Y,W,H} in page coords, or null if
// none matched.
(function () {
  var sels = __SELS__;
  var minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity, n = 0;
  function add(b) {
    if (b.width === 0 || b.height === 0) return;
    n++;
    var x = b.left + window.scrollX, y = b.top + window.scrollY;
    if (x < minX) minX = x;
    if (y < minY) minY = y;
    if (x + b.width > maxX) maxX = x + b.width;
    if (y + b.height > maxY) maxY = y + b.height;
  }
  sels.forEach(function (s) {
    var e = s.indexOf('@button:') === 0
      ? Array.prototype.slice.call(document.querySelectorAll('button,a,[role=button]'))
          .find(function (b) { return (b.textContent || '').trim() === s.slice(8); })
      : document.querySelector(s);
    if (e) add(e.getBoundingClientRect());
  });
  Array.prototype.slice.call(document.querySelectorAll('.rela-anno')).forEach(function (e) {
    add(e.getBoundingClientRect());
  });
  if (n === 0) return null;
  return { X: minX, Y: minY, W: maxX - minX, H: maxY - minY };
})();
