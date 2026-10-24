# -*- coding: utf-8 -*-
"""Verify the live market site: rewired links present in the bundle and actually reachable."""
import re
import urllib.request

base = 'https://plugin.gonavi.org'
html = urllib.request.urlopen(base + '/', timeout=20).read().decode()
src = re.findall(r'src="(/assets/[^"]+\.js)"', html)[0]
body = urllib.request.urlopen(base + src, timeout=20).read().decode('utf-8', 'replace')

checks = {
    '开发者文档 → docs/plugins': 'tree/dev/docs/plugins' in body,
    '社区 → GoNavi Issues': 'github.com/Syngnat/GoNavi/issues' in body,
    '发布插件 → development.md': 'blob/dev/docs/plugins/development.md' in body,
    '页脚 GitHub 链接': 'GoNavi-Plugin-Market' in body,
    '首页/插件分类 kept': '插件分类' in body,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL'), name)

# Reachability of every wired external target.
for url in [
    'https://github.com/Syngnat/GoNavi/tree/dev/docs/plugins',
    'https://github.com/Syngnat/GoNavi/blob/dev/docs/plugins/development.md',
    'https://github.com/Syngnat/GoNavi/issues',
    'https://github.com/Syngnat/GoNavi-Plugin-Market',
]:
    req = urllib.request.Request(url, method='HEAD', headers={'User-Agent': 'market-verify'})
    status = urllib.request.urlopen(req, timeout=20).status
    print('PASS' if status == 200 else 'FAIL', f'reachable {status}', url.rsplit('/', 1)[-1] or url)
