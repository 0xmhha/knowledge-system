import copy
import importlib.util
import json
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parent.parent
SPEC = importlib.util.spec_from_file_location('approved_sf', ROOT/'scripts/b0-apply-semantic-approval.py')
MOD = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MOD)


class FinalFactApprovalTests(unittest.TestCase):
    def setUp(self):
        self.book = json.loads(MOD.SF.MAT.DEFAULT.read_bytes())

    def test_only_declared_partition_changes_inherit_review(self):
        proofs = MOD.final_derivation(self.book)
        self.assertEqual({p['family'] for p in proofs}, {'F-03','F-05','F-06'})
        self.assertTrue(any(p['development_sha256'] != p['final_sha256'] for p in proofs))

    def test_changed_cap_return_test_or_vocabulary_cannot_inherit_review(self):
        for family, path, old, new in [
            ('F-05','project-a/docs/policy.md','10','11'),
            ('F-06','main.go','20','10'),
            ('F-06','main_test.go','TestHealth','TestAcceptance'),
            ('F-03','ontology.yaml','proposed','verified'),
        ]:
            with self.subTest(family=family,path=path):
                altered=copy.deepcopy(self.book)
                fixture=next(f for f in altered['fixtures'] if f['id']==family+'-FINAL')
                self.assertIn(old,fixture['sources'][path])
                fixture['sources'][path]=fixture['sources'][path].replace(old,new)
                with self.assertRaisesRegex(ValueError,'unreviewed test fact'): MOD.final_derivation(altered)


if __name__=='__main__':
    unittest.main()
