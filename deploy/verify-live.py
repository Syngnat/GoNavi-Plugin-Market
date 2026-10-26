# -*- coding: utf-8 -*-
"""Verify the live site's docs center, community link and deployed Markdown assets."""
import re
import urllib.request

base = 'https://plugin.gonavi.org'
html = urllib.request.urlopen(base + '/', timeout=20).read().decode()
src = re.findall(r'src="(/assets/[^"]+\.js)"', html)[0]
body = urllib.request.urlopen(base + src, timeout=20).read().decode('utf-8', 'replace')

checks = {
    '社区 → Discussions #1582': 'discussions/1582' in body,
    '开发者文档 → 站内文档中心': 'to:"/docs"' in body and '/docs/author-guide' in body,
    '旧 GitHub 插件文档链接已移除': 'tree/dev/docs/plugins' not in body and 'blob/dev/docs/plugins' not in body,
    '旧 Issues 社区链接已换': 'GoNavi/issues' not in body.replace('github.com/Syngnat/GoNavi/issues', ''),
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL'), name)

docs = {
    '/docs': '',
    '/docs/author-guide': '# 第三方插件作者指南',
    '/docs/faq': '# 插件作者 FAQ',
    '/docs/v2-api': '# 插件市场 v2：账号与发布闭环（源码重建契约）',
}
for path, marker in docs.items():
    try:
        html = urllib.request.urlopen(base + path, timeout=20).read()
        ok = b'id="root"' in html
        if marker:
            slug = path.rsplit('/', 1)[-1]
            markdown = urllib.request.urlopen(base + f'/docs/{slug}.md', timeout=20).read().decode('utf-8', 'replace')
            ok = ok and marker in markdown
        print(('PASS' if ok else 'FAIL'), f'站内文档 {path}')
    except Exception:
        print('FAIL', f'站内文档 {path}')

req = urllib.request.Request('https://github.com/Syngnat/GoNavi/discussions/1582', method='HEAD', headers={'User-Agent': 'market-verify'})
status = urllib.request.urlopen(req, timeout=20).status
print('PASS' if status == 200 else 'FAIL', f'discussion reachable: {status}')
