# -*- coding: utf-8 -*-
"""Verify live site: community links point at the anchor discussion and it is reachable."""
import re
import urllib.request

base = 'https://plugin.gonavi.org'
html = urllib.request.urlopen(base + '/', timeout=20).read().decode()
src = re.findall(r'src="(/assets/[^"]+\.js)"', html)[0]
body = urllib.request.urlopen(base + src, timeout=20).read().decode('utf-8', 'replace')

checks = {
    '社区 → Discussions #1582': 'discussions/1582' in body,
    '开发者文档 → docs/plugins': 'tree/dev/docs/plugins' in body,
    '发布插件 → development.md': 'blob/dev/docs/plugins/development.md' in body,
    '旧 Issues 社区链接已换': 'GoNavi/issues' not in body.replace('github.com/Syngnat/GoNavi/issues', ''),
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL'), name)

req = urllib.request.Request('https://github.com/Syngnat/GoNavi/discussions/1582', method='HEAD', headers={'User-Agent': 'market-verify'})
status = urllib.request.urlopen(req, timeout=20).status
print('PASS' if status == 200 else 'FAIL', f'discussion reachable: {status}')
