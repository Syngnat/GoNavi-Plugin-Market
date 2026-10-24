# -*- coding: utf-8 -*-
import re
import urllib.request

html = urllib.request.urlopen('https://plugin.gonavi.org/', timeout=20).read().decode()
script = re.findall(r'src="(/assets/[^"]+\.js)"', html)
body = ''.join(
    urllib.request.urlopen('https://plugin.gonavi.org' + src, timeout=20).read().decode('utf-8', 'replace')
    for src in script
)
checks = {
    '开发者社区 removed': '开发者社区' not in body,
    '发布插件 removed': '发布插件' not in body,
    '登录 removed': '>登录<' not in body and '插件市场登录' not in body and '连接账户' not in body,
    '页脚死链 removed': '使用条款' not in body,
    'GitHub link present': 'GoNavi-Plugin-Market' in body,
    '首页/插件分类 kept': '插件分类' in body,
    '搜索/空状态 kept': '搜索插件' in body,
}
for name, ok in checks.items():
    print(('PASS' if ok else 'FAIL'), name)
