"""Scope approval never approves independent gold or rewrites historical results."""
import copy
import unittest
from unittest.mock import patch
import owner_scope_adoption as x

class ScopeDecision(unittest.TestCase):
    def altered(self,name,change):
        read=x.v.read
        def load(p):
            data=read(p)
            if p.name==name:data=copy.deepcopy(data);change(data)
            return data
        return patch.object(x.v,'read',side_effect=load)
    def test_sealed_scope_receipt(self):self.assertEqual(x.check(),[])
    def test_actual_scope_and_history(self):
        d=x.derive();self.assertEqual(d['actual_owner_observations'],40);self.assertFalse(d['original_five_all_real_owner_claim']);self.assertEqual(d['independent_approved'],0)
    def test_scope_approval_cannot_approve_gold(self):
        with self.altered('SCOPE-DECISION.json',lambda d:d.update(independent_gold_approval=True)):
            with self.assertRaisesRegex(ValueError,'promoted'):x.derive()
    def test_different_proposal_not_approved(self):
        with self.altered('SCOPE-DECISION.json',lambda d:d.update(proposal_sha256='0'*64)):
            with self.assertRaisesRegex(ValueError,'decision changed'):x.derive()
    def test_missing_owner_schedule_not_complete(self):
        def missing(d):
            if 'slots'in d:d['slots'][0]['observations'].pop()
        with self.altered('SUMMARY.json',missing):
            with self.assertRaisesRegex(ValueError,'schedules missing'):x.derive()
    def test_history_must_not_be_relabelled_accepted(self):
        def promote(d):
            if 'full12_requirement_acceptance'in d:d['full12_requirement_acceptance']=True
        with self.altered('SUMMARY.json',promote):
            with self.assertRaisesRegex(ValueError,'rewritten'):x.derive()
if __name__=='__main__':unittest.main()
