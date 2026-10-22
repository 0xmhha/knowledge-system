"""Reject a WI-V1 reviewer packet the AI pre-filled, weakened or detached from its sources."""
import copy
import unittest
import wiv1_reviewer_packet as x


class Packet(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.review = x.read(x.ROOT/x.PACKET/'REVIEW-PACKET.json')
        cls.declaration = x.read(x.ROOT/x.PACKET/'INDEPENDENCE-DECLARATION.json')

    def rejects(self, change, target='review'):
        review, declaration = copy.deepcopy(self.review), copy.deepcopy(self.declaration)
        change(review if target == 'review' else declaration)
        with self.assertRaises(ValueError):
            x.inspect(review, declaration)

    def test_actual_packet(self):
        self.assertEqual(x.check(), [])
        self.assertEqual(x.inspect(self.review, self.declaration), dict(contracts=12, matching_rules=11, cited_packets=6))

    def test_ai_cannot_fill_contract_decision(self):
        self.rejects(lambda r: r['contracts'][0]['reviewer'].update(decision='accept'))

    def test_ai_cannot_fill_matching_decision(self):
        self.rejects(lambda r: r['matching_rules'][0]['reviewer'].update(decision='accept'))

    def test_ai_cannot_fill_final_inputs(self):
        self.rejects(lambda r: r['final_inputs']['reviewer'].update(fresh_FINAL_tickets=2))

    def test_ai_cannot_fill_declaration(self):
        self.rejects(lambda d: d['declaration'].update(not_author_of_candidate_outputs=True), 'declaration')

    def test_draft_text_cannot_be_weakened(self):
        self.rejects(lambda r: r['contracts'][1].update(ai_draft_unknown=''))

    def test_remaining_gap_cannot_be_dropped(self):
        self.rejects(lambda r: r['contracts'][2].update(stage95_remaining=''))

    def test_exposure_cannot_be_hidden(self):
        self.rejects(lambda r: r['exposure_notice']['candidate_outputs'].pop())

    def test_contract_cannot_be_removed(self):
        self.rejects(lambda r: r['contracts'].pop())

    def test_packet_cannot_claim_review(self):
        self.rejects(lambda r: r.update(reviewer_filled=True))

    def test_stale_source_rejected(self):
        self.rejects(lambda r: r['sources']['requirements'].update(sha256='0'*64))


if __name__ == '__main__':
    unittest.main()
