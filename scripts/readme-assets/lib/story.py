"""Turns the recorded command output into the demo storyboard (story.json).

    python3 lib/story.py <work-dir>

Every output line is taken verbatim from <work-dir>/out; only the absolute
path of the scratch billing repository is shown as ~/acme/billing.
"""
import json
import re
import sys

WORK = sys.argv[1]
OUT = WORK + '/out'
BILLING = open(OUT + '/billing-path.txt').read().strip()


def read(name):
    text = open(f'{OUT}/{name}').read().replace(BILLING, '~/acme/billing')
    return text.rstrip('\n').split('\n')


def seg(line):
    """Split one output line into colored segments: [[css-class, text], ...]."""
    m = re.match(r'^(  )(create|gen|update)(\s+)(.*)$', line)
    if m:
        cls = {'create': 'teal', 'gen': 'gold', 'update': 'teal'}[m.group(2)]
        return [['', m.group(1)], [cls, m.group(2)], ['', m.group(3)], ['ink', m.group(4)]]
    m = re.match(r'^([⚠✖] \[[a-z_]+\])( [^:]+:)(.*)$', line)
    if m:
        cls = 'warn b' if line.startswith('⚠') else 'err b'
        return [[cls, m.group(1)], ['ink', m.group(2)], ['muted', m.group(3)]]
    m = re.match(r'^Checked .*: (\d+) error\(s\), (\d+) warning\(s\)\.$', line)
    if m:
        cls = 'err b' if m.group(1) != '0' else ('warn b' if m.group(2) != '0' else 'ok b')
        return [[cls, line]]
    m = re.match(r'^(  - )(.*: )(\S+)( → )(\S+)$', line)
    if m:
        return [['muted', m.group(1)], ['ink', m.group(2)], ['muted', m.group(3)],
                ['muted', m.group(4)], ['gold b', m.group(5)]]
    if line.startswith('  - '):
        return [['muted', '  - '], ['ink', line[4:]]]
    if line.endswith('is out of date:'):
        return [['warn b', line]]
    if line.startswith('-') and not line.startswith('---'):
        return [['err', line]]
    if line.startswith('+') and not line.startswith('+++'):
        return [['ok', line]]
    if line.startswith(("Ma'at initialized", 'Next:', 'Run `', 'Optional:')):
        return [['muted', line]]
    if line.startswith('  [!['):  # the README badge snippet init offers
        return [['teal', line]]
    return [['ink', line]]


def front_matter(line):
    if line == '---':
        return [['muted', line]]
    m = re.match(r'^(\s*-?\s*)([a-z_]+:)(.*)$', line)
    if m:
        return [['', m.group(1)], ['teal', m.group(2)], ['ink', m.group(3)]]
    return [['ink b', line]]


def out(name, per=0.035, keep=None):
    lines = read(name)
    if keep:
        lines = [line for line in lines if keep(line)]
    if lines and lines[0] == '':
        lines = lines[1:]
    return {'k': 'out', 'lines': [seg(line) for line in lines], 'per': per}


story = [
    {'k': 'cmd', 'text': 'maat init . --name Billing --summary "Invoices and payments."'},
    out('init.txt', per=0.045),
    {'k': 'pause', 's': 2.6},  # long enough to read the closing hints and badge offer
    {'k': 'clear'},
    {'k': 'comment', 'text': '# describe the code, and the contracts it depends on'},
    {'k': 'cmd', 'text': 'cat docs/architecture/modules/auth.md'},
    {'k': 'out', 'lines': [front_matter(line) for line in read('auth.md')], 'per': 0.05},
    {'k': 'cmd', 'text': 'maat contracts update'},
    out('update1.txt'),
    {'k': 'cmd', 'text': 'maat check'},
    out('check1.txt'),
    {'k': 'pause', 's': 1.6},
    {'k': 'clear'},
    {'k': 'comment', 'text': '# later: a teammate makes tokens last an hour. Code only, no docs.'},
    {'k': 'cmd', 'text': 'git diff'},
    out('diff.txt', keep=lambda line: line.startswith(('-const', '+const'))),
    {'k': 'cmd', 'text': 'git commit -qam "Tokens last an hour"'},
    {'k': 'cmd', 'text': 'maat check'},
    out('check2.txt'),
    {'k': 'pause', 's': 2.2},
    {'k': 'clear'},
    {'k': 'comment', 'text': '# meanwhile, the ledger team ships ledger-events 2.0.0'},
    {'k': 'cmd', 'text': 'maat contracts update --check'},
    out('update-check.txt'),
    {'k': 'pause', 's': 0.9},
    {'k': 'cmd', 'text': 'maat contracts update'},
    out('update2.txt'),
    {'k': 'cmd', 'text': 'maat check'},
    out('check3.txt', per=0.12),
    {'k': 'pause', 's': 2.8},
    {'k': 'end'},
]
json.dump(story, open(WORK + '/story.json', 'w'), ensure_ascii=False, indent=1)
