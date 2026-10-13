"""Validate current prose inventory and local links without changing frozen evidence."""
from pathlib import Path
import re
from urllib.parse import unquote, urlsplit

PROSE_SUFFIXES = {'.md', '.rst', '.adoc'}
ROLES = {'reference', 'contract', 'workflow', 'agent_instructions', 'domain_data', 'fixture', 'frozen_evidence'}
CURRENT_ROLES = {'reference', 'contract', 'workflow', 'agent_instructions'}


def data_role(path):
    if path.startswith('system/eval/') or '/eval/' in path and path.startswith('projects/'):
        return 'frozen_evidence'
    if path.startswith('projects/') and '/domain-knowledge/' in path:
        return 'domain_data'
    if '/testdata/' in path or path.startswith('testdata/') or '/corpus/' in path:
        return 'fixture'
    return None


def validate_catalog(root, catalog, inventory):
    findings = []
    if catalog.get('schema_version') != 1 or not isinstance(catalog.get('documents'), dict):
        return [('docs/DOCUMENTATION-CATALOG.json', 1, 'invalid documentation catalog')]
    rows = catalog['documents']
    for path in sorted(set(inventory) - set(rows)):
        findings.append((path, 1, 'prose is missing from the documentation catalog'))
    for path, row in rows.items():
        p = Path(path)
        if p.is_absolute() or '..' in p.parts or p.suffix.lower() not in PROSE_SUFFIXES:
            findings.append((path, 1, 'invalid catalog path'))
            continue
        if not (root / p).is_file() or path not in inventory:
            findings.append((path, 1, 'catalog document is missing'))
        role = row.get('role') if isinstance(row, dict) else None
        if role not in ROLES:
            findings.append((path, 1, 'invalid documentation role'))
        expected = data_role(path)
        if expected is not None and role != expected:
            findings.append((path, 1, f'retained data must use role {expected}'))
        if expected is None and role not in CURRENT_ROLES:
            findings.append((path, 1, 'current prose cannot bypass checks as retained data'))
    return findings


def unfenced(text):
    """Keep line coordinates while omitting examples inside fenced code."""
    lines = []
    fence = None
    for line in text.splitlines():
        match = re.match(r'^\s*(`{3,}|~{3,})', line)
        if match and fence is None:
            fence = match.group(1)[0]
            lines.append('')
        elif match and match.group(1)[0] == fence:
            fence = None
            lines.append('')
        else:
            lines.append('' if fence else line)
    return '\n'.join(lines)


def heading_ids(text):
    seen = {}
    ids = set(re.findall(r'<a\s+(?:name|id)=["\']([^"\']+)', text, re.I))
    for line in unfenced(text).splitlines():
        match = re.match(r'^\s{0,3}#{1,6}\s+(.+?)\s*#*\s*$', line)
        if not match:
            continue
        label = re.sub(r'<[^>]*>', '', match.group(1))
        label = re.sub(r'\[([^\]]*)\]\([^)]*\)', r'\1', label)
        slug = re.sub(r'[^\w\- ]', '', label.lower()).replace(' ', '-')
        suffix = seen.get(slug, 0)
        seen[slug] = suffix + 1
        ids.add(slug + (f'-{suffix}' if suffix else ''))
    return ids


def markdown_links(text):
    for number, line in enumerate(unfenced(text).splitlines(), 1):
        # Inline code documents examples, not navigable links.
        line = re.sub(r'`[^`]*`', '', line)
        for match in re.finditer(r'!?\[[^\]]*\]\((<[^>]+>|[^\s)]+)(?:\s+["\'][^"\']*["\'])?\)', line):
            yield number, match.group(1).strip('<>')
        match = re.match(r'^\s*\[[^\]]+\]:\s*(<[^>]+>|\S+)', line)
        if match:
            yield number, match.group(1).strip('<>')


def validate_links(root, path, text, repository_files=None):
    findings = []
    for lineno, link in markdown_links(text):
        parsed = urlsplit(link)
        if parsed.scheme or parsed.netloc:
            continue
        target = (root / path).parent / unquote(parsed.path) if parsed.path else root / path
        if not target.exists():
            findings.append((path, lineno, f'local Markdown target does not exist: {link}'))
            continue
        if repository_files is not None:
            try:
                relative = target.resolve().relative_to(root.resolve()).as_posix()
            except ValueError:
                relative = None
            if relative is not None and target.is_dir() and not any(
                    relative == '.' or p.startswith(relative.rstrip('/') + '/') for p in repository_files):
                findings.append((path, lineno, f'local Markdown directory has no repository files: {link}'))
                continue
        if parsed.fragment and target.is_file() and target.suffix.lower() == '.md':
            if unquote(parsed.fragment) not in heading_ids(target.read_text(encoding='utf-8')):
                findings.append((path, lineno, f'Markdown heading does not exist: {link}'))
    return findings
