"""Reject a D04 role assignment that declares, approves or widens more than the user assigned."""
import copy
import unittest
import d04_role_assignment as x
import guardian_deadline
import viewer_readiness as v


class Assignment(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.docs = [x.read(x.ROOT/p) for p in (x.RECORD, x.PROTOCOL, x.PROPOSAL, x.WORKLIST)]

    def rejects(self, index, change):
        docs = copy.deepcopy(self.docs)
        change(docs[index])
        with self.assertRaises(ValueError):
            x.inspect(*docs)

    def d04(self, proposal):
        return next(d for d in proposal['decisions'] if d['id'] == 'D04')

    def test_actual_assignment(self):
        self.assertEqual(x.check(), [])
        self.assertEqual(x.inspect(*self.docs)['pilot'], ['WI-V1'])

    def test_ai_cannot_declare_independence(self):
        self.rejects(0, lambda r: r['roles'].update(independence_declaration='independent'))

    def test_protocol_cannot_declare_independence(self):
        self.rejects(1, lambda p: p['roles'].update(independence_declaration='independent'))

    def test_f05_cannot_be_approved(self):
        self.rejects(1, lambda p: next(r for r in p['freeze_requirements'] if r['id'] == 'F-05').update(decision_status='approved'))

    def test_reviewer_and_acceptor_cannot_merge(self):
        self.rejects(0, lambda r: r['roles'].update(final_scope_acceptor='gold-reviewer'))

    def test_ai_cannot_become_final_gold_author(self):
        self.rejects(0, lambda r: r['roles']['gold_author'].update(final_authority='AI'))

    def test_real_name_cannot_be_recorded(self):
        self.rejects(0, lambda r: r.update(real_name_recorded=True))

    def test_pilot_cannot_widen(self):
        self.rejects(0, lambda r: r.update(pilot_scope=['WI-V1', 'WI-1']))

    def test_formal_acceptance_cannot_be_counted(self):
        self.rejects(3, lambda w: w.update(formal_completed=1))

    def test_execution_cannot_be_enabled(self):
        self.rejects(1, lambda p: p.update(execution_allowed=True))

    def test_receipt_text_must_match(self):
        self.rejects(2, lambda p: self.d04(p)['assignment_receipt'].update(text='모두 승인'))

    def test_stale_worklist_reference_rejected(self):
        self.rejects(3, lambda w: w['d04_role_assignment'].update(sha256='0'*64))


class CurrentGuard(unittest.TestCase):
    """The current readiness guard admits the D04 fields and nothing else."""

    def indices(self, change=None):
        protocol, ledger = (x.read(x.ROOT/p) for p in v.PATHS[:2])
        if change:
            change(protocol)
        return guardian_deadline.current_indices(x.ROOT, protocol, ledger)

    def test_assignment_admitted(self):
        self.assertEqual(self.indices(), [])

    def test_assignment_mismatch_rejected(self):
        self.assertIn('protocol roles differ from the record', self.indices(lambda p: p['roles'].update(final_scope_acceptor='gold-reviewer')))

    def test_other_freeze_decision_still_rejected(self):
        def change(protocol):
            next(r for r in protocol['freeze_requirements'] if r['id'] == 'F-01').update(decision_status='approved')
        self.assertIn('prior decision/gold changed', self.indices(change))


if __name__ == '__main__':
    unittest.main()
