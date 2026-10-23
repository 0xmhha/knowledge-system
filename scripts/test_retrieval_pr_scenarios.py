"""Reject PR scenarios whose answer, sample or prompt does not follow the retrieval evaluation spec."""
import json
import subprocess
import unittest
from pathlib import Path

import retrieval_pr_scenarios as x

DECLSPANS = Path(__file__).with_name('retrieval_declspans.go')
SOURCE = """package p

// One is a documented constant.
const One = 1

const (
	A = 1
	B = 2
)

// T is a type.
type T struct {
	f int
}

// M is a method on a generic-free pointer receiver.
func (t *T) M() {
	_ = t.f
}

func F() {}
"""


def cand(number, intent, base_time=0):
    return x.Candidate(number, f't{number}', '', intent, f'm{number}', f'b{number}', base_time)


class Rules(unittest.TestCase):
    def test_intent_requires_one_mapped_label(self):
        cases = [
            (['Type: Bug'], 'bug_fix'),
            (['Type: Feature', 'Type: API Change'], 'feature_add'),
            (['Type: Bug', 'Type: Feature'], None),
            (['Type: Meta'], None),
            ([], None),
        ]
        for labels, want in cases:
            with self.subTest(labels=labels):
                self.assertEqual(x.intent_for(labels), want)

    def test_file_roles(self):
        cases = [('a/b.go', False, 'code'), ('a/b_test.go', False, 'test'), ('a/b.pb.go', False, 'generated'),
                 ('a/b.go', True, 'generated'), ('README.md', False, 'doc'), ('go.mod', False, 'other')]
        for path, generated, want in cases:
            with self.subTest(path=path):
                self.assertEqual(x.file_role(path, generated), want)

    def test_exclusions(self):
        cases = [
            ('fix x', None, {'code': 1}, 'no single mapped Type label'),
            ('Cherrypick #1 to v1.2.x', 'bug_fix', {'code': 1}, 'cherry-pick, ci, dependency or release PR'),
            ('fix x', 'bug_fix', {'code': 11}, 'more than 10 non-test Go files'),
            ('fix x', 'bug_fix', {'generated': 2}, 'generated-only'),
            ('fix x', 'bug_fix', {'test': 2}, 'no non-test Go file changed'),
            ('add tests', 'test_add', {'test': 2}, None),
            ('fix x', 'bug_fix', {'code': 10}, None),
        ]
        for title, intent, roles, want in cases:
            with self.subTest(title=title, roles=roles):
                self.assertEqual(x.exclusion_reason(title, intent, roles), want)

    def test_release_branch_cherry_pick_excluded(self):
        self.assertEqual(x.exclusion_reason('fix x (#1)', 'bug_fix', {'code': 1}, 'v1.80.x'), 'cherry-pick to a release branch')


class Diff(unittest.TestCase):
    DIFF = '\n'.join([
        'diff --git a/a.go b/a.go', 'index 1..2 100644', '--- a/a.go', '+++ b/a.go',
        '@@ -10,2 +10,3 @@', '-x', '-y', '+x', '+y', '+z',
        '@@ -20,0 +22 @@', '+w',
        'diff --git a/new.go b/new.go', 'new file mode 100644', '@@ -0,0 +1,3 @@', '+package a',
    ])

    def test_pre_image_lines_and_insertions(self):
        a, new = x.parse_diff(self.DIFF)
        self.assertEqual((a.path, a.old_lines, a.insert_after, a.added), ('a.go', (10, 11), (20,), False))
        self.assertTrue(new.added)

    def test_touched_declarations(self):
        decls = [x.Decl('func', 'F', '', 5, 12, 'func F() {'), x.Decl('func', 'G', '', 15, 25, 'func G() {'),
                 x.Decl('func', 'H', '', 26, 30, 'func H() {')]
        change = x.FileChange('a.go', (10, 11), (20, 25), False, False)
        # 10-11 edit F; insertion after 20 lands inside G; insertion after 25 sits between G and H.
        self.assertEqual([d.name for d in x.touched_decls(decls, change)], ['F', 'G'])


class Sampling(unittest.TestCase):
    def test_round_robin_caps_and_order(self):
        pool = [cand(n, 'bug_fix') for n in range(1, 60)] + [cand(100 + n, 'refactor') for n in range(5)]
        picked = x.sample(pool)
        intents = [c.intent for c in picked]
        self.assertEqual(intents.count('bug_fix'), x.MAX_PER_INTENT)
        self.assertEqual(intents.count('refactor'), 5)
        self.assertEqual([c.number for c in picked], sorted(c.number for c in picked))
        self.assertEqual(min(c.number for c in picked if c.intent == 'bug_fix'), 1)

    def test_total_cap(self):
        pool = [cand(i * 1000 + n, intent) for i, intent in enumerate(x.INTENT_ORDER) for n in range(40)]
        self.assertEqual(len(x.sample(pool)), x.MAX_SAMPLE)

    def test_snapshot_groups_are_time_contiguous(self):
        pool = [cand(n, 'bug_fix', base_time=100 - n) for n in range(12)]
        groups = x.snapshot_groups(pool, 5)
        times = [[c.base_time for c in g] for g in groups]
        self.assertLessEqual(len(groups), 5)
        self.assertEqual(sum(times, []), sorted(sum(times, [])))


class Leaks(unittest.TestCase):
    TRUTH = [x.Truth('balancer/pickfirst/pickfirstleaf.go', 'func', 'updateSubConnState', 'pickfirstBalancer', 1, 9, '')]

    def test_answer_names_rejected(self):
        for prompt in ['updateSubConnState 가 상태를 놓쳐', 'pickfirstBalancer 에서 상태가 늦게 바뀌어',
                       'PickFirstLeaf 파일 쪽 문제', 'UPDATESUBCONNSTATE 를 고쳐 줘']:
            with self.subTest(prompt=prompt):
                self.assertTrue(x.leaked_terms(prompt, self.TRUTH))

    def test_behaviour_prompt_allowed(self):
        self.assertEqual(x.leaked_terms('첫 번째로 고른 연결이 끊긴 뒤 다른 주소로 넘어가지 않는 문제를 고치고 싶어', self.TRUTH), [])

    def test_file_named_after_its_package_is_a_package_word(self):
        truth = [x.Truth('encoding/gzip/gzip.go', 'func', 'Decompress', 'compressor', 1, 9, '')]
        self.assertEqual(x.leaked_terms('gzip 압축 해제 쪽 정리', truth), [])
        self.assertEqual(x.leaked_terms('credentials 쪽 tls 검증', [x.Truth('credentials/tls.go', 'func', 'Verify', '', 1, 2, '')]), ['tls'])

    def test_package_words_allowed(self):
        self.assertEqual(x.leaked_terms('balancer 쪽에서 연결 상태가 늦게 반영돼', self.TRUTH), [])


class Assemble(unittest.TestCase):
    def test_scenario_has_expected_intent_not_intent(self):
        c = {'number': 7, 'title': 't', 'intent': 'bug_fix', 'snapshot': 'a' * 40,
             'truth': [{'file': 'a.go', 'start_line': 3, 'end_line': 9, 'anchor': 'func F() {'}]}
        text = x.scenario_yaml(c, '질문')
        self.assertIn('expected_intent: bug_fix', text)
        self.assertNotIn('\nintent:', text)
        self.assertIn('expected_commit: ' + 'a' * 40, text)


class DeclSpans(unittest.TestCase):
    """The answer ranges come from the Go parser, doc comments included."""

    def test_ranges(self):
        out = subprocess.run(['go', 'run', str(DECLSPANS)], input=SOURCE, capture_output=True, text=True, timeout=300, check=True).stdout
        got = [(d['kind'], d.get('recv', ''), d['name'], d['start_line'], d['end_line']) for d in map(json.loads, out.splitlines())]
        self.assertEqual(got, [('value', '', 'One', 3, 4), ('value', '', 'A', 7, 7), ('value', '', 'B', 8, 8),
                               ('type', '', 'T', 11, 14), ('func', 'T', 'M', 16, 19), ('func', '', 'F', 21, 21)])


if __name__ == '__main__':
    unittest.main()
