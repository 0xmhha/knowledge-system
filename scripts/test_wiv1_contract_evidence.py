"""Reject a WI-V1 contract evidence review that cites, quotes or concludes more than it holds."""
import copy
import unittest
import wiv1_contract_evidence as x


class Review(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.review = x.read(x.ROOT/x.PACKET/'REVIEW.json')

    def rejects(self, change):
        review = copy.deepcopy(self.review)
        change(review)
        with self.assertRaises(ValueError):
            x.inspect_review(review)

    def row(self, review, ident):
        return next(r for r in review['requirements'] if r['id'] == ident)

    def test_actual_review(self):
        self.assertEqual(x.check(), [])
        d = x.inspect_review(self.review)
        self.assertEqual((d['contracts'], d['contracts_without_new_evidence']), (12, ['LATE-DRAIN']))
        self.assertEqual(d['cited_packets'], sorted(x.CITABLE))

    def test_contract_cannot_be_promoted(self):
        self.rejects(lambda r: self.row(r, 'METHOD-ACK').update(full_requirement_satisfied=True))

    def test_independent_approval_not_inferred(self):
        self.rejects(lambda r: r.update(independent_approved=1))

    def test_quoted_value_must_match_packet(self):
        def change(r):
            ev = self.row(r, 'METHOD-ACK')['evidence'][1]
            ev['summary_fields']['post_ack_same_generation_uses'] = 1
        self.rejects(change)

    def test_stale_manifest_rejected(self):
        self.rejects(lambda r: self.row(r, 'API-UNSUB')['evidence'][0].update(manifest_sha256='0'*64))

    def test_uncited_packet_rejected(self):
        self.rejects(lambda r: self.row(r, 'API-ENTRY')['evidence'][0].update(packet='ticket-dev-gold-v1'))

    def test_requirement_text_cannot_be_weakened(self):
        self.rejects(lambda r: self.row(r, 'API-UNSUB').update(unknown=''))

    def test_remaining_gap_cannot_be_dropped(self):
        self.rejects(lambda r: self.row(r, 'NORMAL-DELIVERY').update(remaining=''))

    def test_previous_gap_cannot_be_rewritten(self):
        self.rejects(lambda r: self.row(r, 'BLOCKED-DELIVERY').update(previous_remaining='closed'))

    def test_missing_contract_rejected(self):
        self.rejects(lambda r: r['requirements'].pop())

    def test_independent_prerequisites_kept(self):
        self.rejects(lambda r: r.update(blocked_next=[]))


if __name__ == '__main__':
    unittest.main()
