(function () {
  'use strict';

  var LOCALES = window.__BG_LOCALES__ || {};
  var bridge = window.gonavi;
  if (!bridge || typeof bridge.call !== 'function') return;

  var prefs = window.gonaviHostPrefs || { locale: 'en-US', theme: 'light' };
  var locale = prefs.locale || 'en-US';
  var theme = prefs.theme || 'light';
  var palette = prefs.palette || {};

  var MAX_FILE_BYTES = 10 * 1024 * 1024;
  var MAX_DATA_URL_CHARS = 1.5 * 1024 * 1024;
  var MAX_SIDE = 1920;
  var QUALITIES = [0.82, 0.72, 0.62, 0.52, 0.42, 0.32];

  var state = {
    background: null,
    image: null,
    gallery: null,
    galleryOpen: false,
    thumbs: {},
    pendingThumbs: {},
    error: null,
    galleryError: null,
    busy: false,
    polling: null,
    localPosition: null,
  };

  function t(key, params) {
    var table = LOCALES[locale] || LOCALES['en-US'] || {};
    var text = table[key];
    if (text === undefined) text = (LOCALES['en-US'] || {})[key];
    if (text === undefined) return key;
    if (params) {
      Object.keys(params).forEach(function (name) {
        text = text.split('{{' + name + '}}').join(String(params[name]));
      });
    }
    return text;
  }

  function $(id) { return document.getElementById(id); }
  function esc(value) {
    return String(value === undefined || value === null ? '' : value)
      .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;').replace(/'/g, '&#39;');
  }
  function clamp(value, min, max) { return Math.min(max, Math.max(min, value)); }
  function percent(value) { return t('percent', { value: Math.round(value * 10) / 10 }); }

  function applyTheme() {
    var root = document.documentElement;
    var map = {
      '--bg': palette.bg, '--panel': palette.panel, '--text': palette.text,
      '--muted': palette.muted, '--border': palette.border || palette.border1,
      '--accent': palette.accent, '--accent-soft': palette.accentSoft,
      '--warn-bg': palette.warnBg,
    };
    Object.keys(map).forEach(function (name) {
      if (map[name]) root.style.setProperty(name, map[name]);
    });
    document.body.dataset.theme = theme;
  }

  function reportHeight() {
    if (state.resizeScheduled) return;
    state.resizeScheduled = true;
    // Timers rather than rAF: embedded iframes can have rAF throttled when the
    // host pane scrolls them out of view, which would freeze the reported height.
    setTimeout(function () {
      state.resizeScheduled = false;
      if (typeof bridge.resize === 'function') {
        bridge.resize(document.documentElement.scrollHeight + 2);
      }
    }, 0);
  }

  function call(method, args) {
    return Promise.resolve(bridge.call(method, args || {}));
  }

  function setError(key) {
    state.error = key || null;
    var node = $('error');
    node.hidden = !key;
    node.textContent = key ? t(key) : '';
    reportHeight();
  }

  function reportFailure(error, fallbackKey) {
    var message = error && error.message ? String(error.message) : '';
    if (message.indexOf('gallery_in_use') >= 0) { setError('gallery.remove_in_use'); return; }
    if (message.indexOf('gallery_busy') >= 0) { setError('gallery.busy'); return; }
    setError(fallbackKey || 'error.operation_failed');
  }

  function backgroundLabel() {
    var bg = state.background;
    if (!bg || !bg.hasImage) return '';
    return bg.imageName || '';
  }

  function currentPosition() {
    return state.localPosition || { x: (state.background && state.background.positionX) || 0, y: (state.background && state.background.positionY) || 0 };
  }

  function currentImageURL() {
    if (state.image && state.image.dataUrl) return state.image.dataUrl;
    var bg = state.background;
    if (bg && bg.resolved) return bg.resolved.previewUrl || bg.resolved.url;
    return '';
  }

  function render() {
    var bg = state.background;
    var hasSource = !!(bg && (bg.hasImage || bg.builtin));

    $('enabled-label').textContent = t('enabled');
    $('enabled-hint').textContent = t('enabled_hint');
    $('enabled').checked = !!(bg && bg.enabled);
    $('enabled').disabled = state.busy || !hasSource;

    $('gallery-toggle').textContent = t(state.galleryOpen ? 'gallery.close' : 'gallery.open');
    $('gallery-toggle').setAttribute('aria-expanded', state.galleryOpen ? 'true' : 'false');
    $('gallery').hidden = !state.galleryOpen;
    $('gallery-title').textContent = t('gallery.title');
    $('gallery-description').textContent = t('gallery.description');
    $('gallery-refresh').textContent = t('gallery.refresh');
    $('gallery-refresh').disabled = state.busy;

    $('description').textContent = t('description');
    $('empty').hidden = hasSource;
    $('empty').textContent = t('empty');
    $('position').hidden = !hasSource;

    $('upload').textContent = t(hasSource ? 'replace' : 'upload');
    $('upload').disabled = state.busy;
    $('remove').hidden = !hasSource;
    $('remove').textContent = t('remove');
    $('remove').disabled = state.busy;
    $('reset').textContent = t('reset');
    $('reset').disabled = state.busy || (!hasSource && bg && bg.opacity === 15);

    $('opacity-label').textContent = t('intensity');
    $('opacity').value = String((bg && bg.opacity) || 15);
    $('opacity').disabled = state.busy;
    $('opacity-out').textContent = percent((bg && bg.opacity) || 15);

    if (hasSource) {
      var position = currentPosition();
      var ratio = (bg && bg.viewportRatio) || (16 / 9);
      $('preview').style.aspectRatio = String(ratio);
      var image = $('preview-image');
      var url = currentImageURL();
      if (url && image.getAttribute('src') !== url) image.setAttribute('src', url);
      image.style.objectFit = (bg && bg.resolved && bg.resolved.fit) || 'cover';
      image.style.transform = 'translate(' + position.x + '%, ' + position.y + '%)';
      image.alt = t('preview', { name: backgroundLabel() || '' });
      $('image-name').textContent = backgroundLabel();
      $('position-hint').textContent = t('position.hint');
      $('pos-x-label').textContent = t('position.horizontal');
      $('pos-y-label').textContent = t('position.vertical');
      $('pos-x').value = String(position.x);
      $('pos-y').value = String(position.y);
      $('pos-x-out').textContent = percent(position.x);
      $('pos-y-out').textContent = percent(position.y);
      $('pos-x').disabled = state.busy;
      $('pos-y').disabled = state.busy;
      $('position-center').textContent = t('position.center');
      $('position-center').disabled = state.busy || (position.x === 0 && position.y === 0);
    }

    renderGallery();
    reportHeight();
  }

  function galleryEntries() {
    return (state.gallery && state.gallery.entries) || [];
  }

  function entryName(entry) {
    return entry.names[locale] || entry.names['en-US'] || entry.names['zh-CN'] || entry.id;
  }

  function activeProgress(entry) {
    var records = [];
    if (state.gallery && state.gallery.download) records.push(state.gallery.download);
    if (state.gallery && state.gallery.selection) records.push(state.gallery.selection);
    for (var i = 0; i < records.length; i += 1) {
      var record = records[i];
      if (record.id === entry.id && record.version === entry.version && !record.done) return record;
    }
    return null;
  }

  function renderGallery() {
    if (!state.galleryOpen) return;
    var gallery = state.gallery;
    var banner = $('gallery-banner');
    var notices = [];
    if (state.galleryError) notices.push(t(state.galleryError));
    if (gallery && gallery.error) notices.push(t('gallery.error.' + gallery.error) || t('gallery.error.unavailable'));
    if (gallery && gallery.stale) notices.push(t('gallery.stale'));
    if (gallery && gallery.warningCode) notices.push(t('gallery.error.' + gallery.warningCode) || t('gallery.error.unavailable'));
    banner.hidden = notices.length === 0;
    banner.textContent = notices.join('；');

    $('gallery-loading').hidden = !!(gallery && gallery.entries && gallery.entries.length) || !!state.galleryLoaded;
    $('gallery-loading').textContent = t('gallery.loading');
    var entries = galleryEntries();
    $('gallery-empty').hidden = entries.length > 0;
    $('gallery-empty').textContent = t('gallery.empty');

    var html = entries.map(function (entry) {
      var key = entry.id + '@' + entry.version;
      var thumb = state.thumbs[key];
      var progress = activeProgress(entry);
      var actions = [];
      if (progress) {
        actions.push('<button class="bg-btn small" data-action="cancel" data-id="' + esc(entry.id) + '" data-version="' + esc(entry.version) + '">' + esc(t('gallery.cancel')) + '</button>');
      } else if (entry.selected) {
        actions.push('<span class="bg-badge">' + esc(t('gallery.selected')) + '</span>');
      } else if (entry.downloaded) {
        actions.push('<button class="bg-btn small primary" data-action="select" data-id="' + esc(entry.id) + '" data-version="' + esc(entry.version) + '">' + esc(t('gallery.use')) + '</button>');
        actions.push('<button class="bg-btn small danger" data-action="remove" data-id="' + esc(entry.id) + '" data-version="' + esc(entry.version) + '">' + esc(t('gallery.remove')) + '</button>');
      } else {
        // Matches the previous built-in panel: one click downloads and applies.
        actions.push('<button class="bg-btn small" data-action="select" data-id="' + esc(entry.id) + '" data-version="' + esc(entry.version) + '">' + esc(t('gallery.download')) + '</button>');
      }
      var badge = entry.selected ? '' : (entry.downloaded ? '<span class="bg-badge downloaded">' + esc(t('gallery.downloaded')) + '</span>' : '');
      var progressHTML = progress
        ? '<div class="bg-progress" aria-label="' + esc(t('gallery.downloading')) + '"><span style="width:' + progressWidth(progress) + '%"></span></div>'
        : '';
      var license = entry.license && entry.license.name
        ? '<span>' + esc(t('gallery.license')) + '：<button class="bg-btn link" data-action="license" data-url="' + esc(entry.license.url) + '">' + esc(entry.license.name) + '</button>'
          + (entry.license.nonCommercial ? ' · ' + esc(t('gallery.non_commercial')) : '') + '</span>'
        : '';
      return '<div class="bg-card' + (entry.selected ? ' is-selected' : '') + '" role="listitem">'
        + '<div class="bg-card-thumb">' + (thumb ? '<img alt="" src="' + thumb + '">' : '<span class="placeholder">' + esc(entryName(entry)) + '</span>') + '</div>'
        + '<div class="bg-card-body">'
        + '<span class="bg-card-name" title="' + esc(entryName(entry)) + '">' + esc(entryName(entry)) + '</span>'
        + '<span class="bg-card-meta">' + badge + license + '</span>'
        + progressHTML
        + '<span class="bg-card-actions">' + actions.join('') + '</span>'
        + '</div></div>';
    }).join('');
    $('gallery-grid').innerHTML = html;
    var selected = entries.filter(function (entry) { return entry.selected; })[0];
    $('gallery-credit').textContent = selected ? ((selected.attribution || '') + (selected.attribution ? ' · ' : '') + (selected.sourceUrl || '')) : '';
    loadThumbnails();
    reportHeight();
  }

  function progressWidth(record) {
    if (!record.total) return 8;
    return clamp(Math.round(record.downloaded / record.total * 100), 4, 100);
  }

  function loadThumbnails() {
    galleryEntries().forEach(function (entry) {
      var key = entry.id + '@' + entry.version;
      if (state.thumbs[key] || state.pendingThumbs[key]) return;
      state.pendingThumbs[key] = true;
      call('appearance.gallery.thumbnail', { id: entry.id, version: entry.version })
        .then(function (result) {
          delete state.pendingThumbs[key];
          if (result && typeof result.dataUrl === 'string') {
            state.thumbs[key] = result.dataUrl;
            renderGallery();
          }
        })
        .catch(function () { delete state.pendingThumbs[key]; });
    });
  }

  function refreshBackground() {
    return call('appearance.background.state').then(function (result) {
      state.background = result || null;
      if (result && result.hasImage) {
        return call('appearance.background.image').then(function (image) {
          state.image = image && image.dataUrl ? image : null;
        });
      }
      state.image = null;
      return null;
    });
  }

  function refreshGallery(refresh) {
    return call(refresh ? 'appearance.gallery.refresh' : 'appearance.gallery.state').then(function (result) {
      state.gallery = result || null;
      state.galleryLoaded = true;
      state.galleryError = null;
    }).catch(function (error) {
      state.galleryError = 'gallery.error.unavailable';
      reportFailure(error);
    });
  }

  function refreshAll() {
    return Promise.all([refreshBackground(), refreshGallery(false)]).then(render).catch(function (error) {
      setError('error.operation_failed');
      reportFailure(error);
    });
  }

  function startPolling() {
    if (state.polling) return;
    state.polling = setInterval(function () {
      refreshGallery(false).then(function () {
        var download = state.gallery && state.gallery.download;
        var selection = state.gallery && state.gallery.selection;
        var active = (download && !download.done) || (selection && !selection.done);
        render();
        if (!active) {
          stopPolling();
          var failed = (download && download.error) || (selection && selection.error);
          if (failed) setError('error.operation_failed');
          else setError(null);
          refreshBackground().then(function () { return refreshGallery(false); }).then(render);
        }
      }).catch(function () { stopPolling(); });
    }, 600);
  }

  function stopPolling() {
    if (state.polling) { clearInterval(state.polling); state.polling = null; }
  }

  function withBusy(promise) {
    state.busy = true;
    render();
    return promise.finally(function () { state.busy = false; render(); });
  }

  function applyOpacity(value) {
    var next = clamp(Number(value), 5, 30);
    if (state.background) state.background.opacity = next;
    $('opacity-out').textContent = percent(next);
    return withBusy(call('appearance.background.set-opacity', { opacity: next }).catch(function (error) {
      reportFailure(error);
      return refreshBackground().then(render);
    }));
  }

  function commitPosition(x, y) {
    state.localPosition = null;
    if (state.background) {
      state.background.positionX = x;
      state.background.positionY = y;
    }
    return call('appearance.background.set-position', { positionX: x, positionY: y }).catch(function (error) {
      reportFailure(error);
      return refreshBackground().then(render);
    }).then(render);
  }

  function setPosition(x, y) {
    if (!state.background) return;
    state.localPosition = { x: clamp(x, -100, 100), y: clamp(y, -100, 100) };
    render();
    return commitPosition(state.localPosition.x, state.localPosition.y);
  }

  function prepareImage(file) {
    return new Promise(function (resolve, reject) {
      var url = URL.createObjectURL(file);
      var image = new Image();
      image.onload = function () {
        try {
          var scale = Math.min(1, MAX_SIDE / Math.max(image.naturalWidth, image.naturalHeight));
          var canvas = document.createElement('canvas');
          canvas.width = Math.max(1, Math.round(image.naturalWidth * scale));
          canvas.height = Math.max(1, Math.round(image.naturalHeight * scale));
          var context = canvas.getContext('2d');
          if (!context) throw new Error('canvas');
          context.drawImage(image, 0, 0, canvas.width, canvas.height);
          for (var i = 0; i < QUALITIES.length; i += 1) {
            var dataUrl = canvas.toDataURL('image/jpeg', QUALITIES[i]);
            if (dataUrl.length <= MAX_DATA_URL_CHARS) { resolve(dataUrl); return; }
          }
          reject({ code: 'too_large' });
        } catch (error) {
          reject({ code: 'invalid_image' });
        } finally {
          URL.revokeObjectURL(url);
        }
      };
      image.onerror = function () {
        URL.revokeObjectURL(url);
        reject({ code: 'invalid_image' });
      };
      image.src = url;
    });
  }

  function handleFile(file) {
    if (!file) return;
    if (!/^image\/(png|jpeg|webp)$/.test(file.type)) { setError('error.invalid_type'); return; }
    if (file.size > MAX_FILE_BYTES) { setError('error.too_large'); return; }
    setError(null);
    withBusy(prepareImage(file).then(function (dataUrl) {
      return call('appearance.background.set-custom', { dataUrl: dataUrl, name: file.name }).then(function () {
        return refreshAll();
      });
    }).catch(function (failure) {
      var code = failure && failure.code;
      setError(code === 'too_large' ? 'error.too_large' : code === 'invalid_image' ? 'error.invalid_image' : 'error.operation_failed');
    }));
  }

  function bind() {
    $('enabled').addEventListener('change', function (event) {
      withBusy(call('appearance.background.set-enabled', { enabled: event.target.checked }).then(refreshBackground).then(render).catch(function (error) {
        reportFailure(error);
        return refreshBackground().then(render);
      }));
    });

    $('gallery-toggle').addEventListener('click', function () {
      state.galleryOpen = !state.galleryOpen;
      render();
      if (!state.galleryOpen) return;
      // First open refreshes the remote catalog, matching the previous built-in panel.
      var first = !state.galleryInitialized;
      state.galleryInitialized = true;
      withBusy(refreshGallery(first).then(render));
    });

    $('gallery-refresh').addEventListener('click', function () {
      withBusy(refreshGallery(true).then(render));
    });

    $('gallery-grid').addEventListener('click', function (event) {
      var target = event.target.closest('[data-action]');
      if (!target) return;
      var action = target.getAttribute('data-action');
      var id = target.getAttribute('data-id');
      var version = target.getAttribute('data-version');
      if (action === 'license') {
        call('system.open-external', { url: target.getAttribute('data-url') }).catch(function () {
          setError('error.operation_failed');
        });
        return;
      }
      if (action === 'select') {
        withBusy(call('appearance.background.select-builtin', { id: id, version: version }).then(function () {
          startPolling();
          return refreshGallery(false);
        }).then(render).catch(function (error) { reportFailure(error); }));
        return;
      }
      if (action === 'download') {
        withBusy(call('appearance.background.select-builtin', { id: id, version: version }).then(function () {
          startPolling();
          return refreshGallery(false);
        }).then(render).catch(function (error) { reportFailure(error); }));
        return;
      }
      if (action === 'cancel') {
        withBusy(call('appearance.gallery.cancel', {}).then(function () {
          stopPolling();
          return Promise.all([refreshGallery(false), refreshBackground()]);
        }).then(render).catch(function (error) { reportFailure(error); }));
        return;
      }
      if (action === 'remove') {
        withBusy(call('appearance.gallery.remove', { id: id, version: version }).then(function () {
          return refreshGallery(false);
        }).then(render).catch(function (error) {
          if (String(error && error.message).indexOf('gallery_in_use') >= 0) {
            setError('gallery.remove_in_use');
          } else {
            reportFailure(error);
          }
        }));
      }
    });

    $('upload').addEventListener('click', function () {
      var input = $('file');
      input.value = '';
      input.click();
    });

    $('file').addEventListener('change', function (event) {
      handleFile(event.target.files && event.target.files[0]);
      event.target.value = '';
    });

    $('remove').addEventListener('click', function () {
      withBusy(call('appearance.background.clear').then(refreshAll));
    });

    $('reset').addEventListener('click', function () {
      withBusy(call('appearance.background.reset').then(refreshAll));
    });

    $('opacity').addEventListener('input', function (event) {
      var value = clamp(Number(event.target.value), 5, 30);
      $('opacity-out').textContent = percent(value);
      if (state.background) state.background.opacity = value;
    });
    $('opacity').addEventListener('change', function (event) { applyOpacity(event.target.value); });

    function previewMove(x, y) {
      if (!state.background) return;
      state.localPosition = { x: clamp(x, -100, 100), y: clamp(y, -100, 100) };
      var position = currentPosition();
      var image = $('preview-image');
      image.style.transform = 'translate(' + position.x + '%, ' + position.y + '%)';
      $('pos-x').value = String(position.x);
      $('pos-y').value = String(position.y);
      $('pos-x-out').textContent = percent(position.x);
      $('pos-y-out').textContent = percent(position.y);
    }

    ['pos-x', 'pos-y'].forEach(function (id) {
      $(id).addEventListener('input', function () {
        previewMove(Number($('pos-x').value), Number($('pos-y').value));
      });
      $(id).addEventListener('change', function () {
        var position = currentPosition();
        commitPosition(position.x, position.y);
      });
    });

    $('position-center').addEventListener('click', function () { setPosition(0, 0); });

    var drag = null;
    var preview = $('preview');
    preview.addEventListener('pointerdown', function (event) {
      if (state.busy || !state.background || (event.button !== undefined && event.button !== 0)) return;
      var rect = preview.getBoundingClientRect();
      if (!rect.width || !rect.height) return;
      event.preventDefault();
      var position = currentPosition();
      drag = { pointerId: event.pointerId, x: event.clientX, y: event.clientY, width: rect.width, height: rect.height, position: position };
      try { preview.setPointerCapture(event.pointerId); } catch (error) { /* document fallback */ }
    });
    document.addEventListener('pointermove', function (event) {
      if (!drag || event.pointerId !== drag.pointerId) return;
      event.preventDefault();
      previewMove(
        drag.position.x + (event.clientX - drag.x) / drag.width * 100,
        drag.position.y + (event.clientY - drag.y) / drag.height * 100
      );
    }, { passive: false });
    function endDrag(event) {
      if (!drag || (event && event.pointerId !== drag.pointerId)) return;
      drag = null;
      var position = currentPosition();
      commitPosition(position.x, position.y);
    }
    document.addEventListener('pointerup', endDrag);
    document.addEventListener('pointercancel', endDrag);
    window.addEventListener('blur', function () { endDrag(); });

    if (typeof ResizeObserver === 'function') {
      new ResizeObserver(reportHeight).observe(document.body);
    }
    window.addEventListener('gonavi:host-preferences', function () {
      prefs = window.gonaviHostPrefs || prefs;
      locale = prefs.locale || locale;
      theme = prefs.theme || theme;
      palette = prefs.palette || palette;
      applyTheme();
      render();
    });
  }

  applyTheme();
  bind();
  refreshAll().then(function () { refreshGallery(false).then(render); });
})();
