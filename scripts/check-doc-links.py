#!/usr/bin/env python3
"""Fail when a docs link or anchor points nowhere.

Checks every /docs/... link and #anchor in site/content/docs against the pages and their headings,
and every relative link in the READMEs against the tree. The site build does not: Next renders a
link to a missing page or heading without complaint, and the reader finds out.

Usage: scripts/check-doc-links.py   (from the repository root)
"""
import glob
import os
import re
import sys

D='site/content/docs/'
def slug(h):
    h=re.sub(r'`','',h).strip().lower()
    h=re.sub(r'[^\w\- ]','',h)
    return h.replace(' ','-')
def page(path):
    p=path.strip('/').removeprefix('docs').strip('/')
    for c in [D+p+'.mdx',D+p+'/index.mdx'] if p else [D+'index.mdx']:
        if os.path.exists(c): return c
anchors={}
def heads(f):
    if f not in anchors:
        anchors[f]={slug(m) for m in re.findall(r'^#{1,6} (.+)$',open(f).read(),re.M)}
    return anchors[f]
bad=0
for f in glob.glob(D+'**/*.mdx',recursive=True):
    s=open(f).read()
    for m in re.finditer(r'\]\((/docs[^)\s]*|#[^)\s]+)\)|href="(/docs[^"]*)"',s):
        u=m.group(1) or m.group(2)
        path,_,a=u.partition('#')
        t=page(path) if path else f
        if not t: print('NOPAGE',f,u); bad+=1; continue
        if a and a not in heads(t): print('NOANCHOR',f,u); bad+=1
# The READMEs link to the published site; check those against the pages too.
for f in ['README.md', 'npm/README.md']:
    for path, a in re.findall(r'https://barakchamo\.github\.io/boxer(/docs[^)#\s]*)(?:#([^)\s]+))?', open(f).read()):
        t = page(path.rstrip('/'))
        if not t:
            print('NOPAGE', f, path); bad += 1
        elif a and a not in heads(t):
            print('NOANCHOR', f, path + '#' + a); bad += 1
for f in ['README.md']+glob.glob('examples/*/README.md')+['examples/README.md','npm/README.md']:
    for u in re.findall(r'\]\(([^)#\s]+)(?:#[^)]*)?\)',open(f).read()):
        if u.startswith('http') or u.startswith('mailto'): continue
        if not os.path.exists(os.path.join(os.path.dirname(f),u)): print('NOFILE',f,u); bad+=1
if bad:
    sys.exit(f'{bad} broken link(s)')
print('every docs link and anchor resolves')
