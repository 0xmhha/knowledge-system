"""Score baseline outputs with the same overlap and dedup rule as cks eval."""
import unittest

import retrieval_baselines as x

C = x.Cite


class Rank(unittest.TestCase):
    def test_first_hit_rank(self):
        want = [C('a.go', 10, 20)]
        cases = [
            ([C('a.go', 15, 15)], 1),
            ([C('b.go', 1, 99), C('a.go', 20, 30)], 2),
            ([C('b.go', 1, 2), C('b.go', 1, 2), C('a.go', 1, 10)], 2),
            ([C('a.go', 21, 30)], 0),
            ([], 0),
            ([C('a.go', 1, x.WHOLE_FILE_END)], 1),
        ]
        for actual, rank in cases:
            with self.subTest(actual=actual):
                self.assertEqual(x.first_hit_rank(want, actual), rank)

    def test_summary_keeps_misses_in_denominator(self):
        s = x.summarize([1, 3, 0, 0])
        self.assertEqual((s['scenarios'], s['hit_at_1'], s['hit_at_3']), (4, 0.25, 0.5))
        self.assertAlmostEqual(s['mrr'], (1 + 1 / 3) / 4)
        self.assertEqual(x.summarize([]), {'scenarios': 0})


class Parse(unittest.TestCase):
    def test_ckg_seeds_then_neighbours_in_order(self):
        md = '\n'.join([
            '## Query: q', '',
            '1. **`Foo`** [Struct] · `p.Foo` · internal/p/foo.go:48',
            '2. **`Bar`** [Method] · `p.Foo.Bar` · internal/p/foo.go:90',
            '### Neighbourhood (BFS)',
            '- `Foo` —[defines]→ `x` · internal/p/foo.go:49',
            '- `Foo` —[defines]→ `internal/p/foo.go` · internal/p/foo.go:1',
            'not a citation line',
        ])
        self.assertEqual(x.parse_ckg(md), [C('internal/p/foo.go', 48, 48), C('internal/p/foo.go', 90, 90),
                                          C('internal/p/foo.go', 49, 49), C('internal/p/foo.go', 1, 1)])

    def test_grep_terms_are_ascii_words_once(self):
        self.assertEqual(x.grep_terms('xDS 클러스터에서 RBAC rbac 규칙과 API 를 바꾸고 싶어 in TLS'), ['xds', 'rbac', 'tls'])
        self.assertEqual(x.grep_terms('연결이 끊긴 뒤 다시 붙을 때'), [])


if __name__ == '__main__':
    unittest.main()
