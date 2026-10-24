"""Insert plugin.gonavi.org into Bero's SNI map (idempotent, asserts the anchor)."""
from pathlib import Path

path = Path('/etc/nginx/stream-conf.d/gonavi-sni.conf')
text = path.read_text()
if 'plugin.gonavi.org' in text:
    print('map already contains plugin.gonavi.org')
else:
    anchor = '    gonavi.org       127.0.0.1:2053;'
    assert anchor in text, 'anchor line missing; refusing to guess'
    text = text.replace(anchor, anchor + '\n    plugin.gonavi.org  127.0.0.1:2053;', 1)
    path.write_text(text)
    print('map updated')
