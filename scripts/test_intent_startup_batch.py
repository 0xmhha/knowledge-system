"""Protect development denominators, cleanup, and batching/source parity."""
import gzip
import json
import tempfile
import unittest
from pathlib import Path
import intent_startup_batch as r
import evidence_revision

class Numeric(unittest.TestCase):
    def test_exact_text_matching_is_order_independent(self):
        a={str(i):[1.0]+[0.0]*1023 for i in range(66)}
        self.assertEqual(r.parity(a,dict(reversed(list(a.items()))))['max_absolute_difference'],0)
    def test_missing_text_or_large_difference_is_rejected(self):
        a={str(i):[1.0]+[0.0]*1023 for i in range(66)}
        with self.assertRaises(ValueError):r.parity(a,{k:v for k,v in a.items() if k!='0'})
        b=dict(a);b['0']=[0.0,1.0]+[0.0]*1022
        with self.assertRaises(ValueError):r.parity(a,b)
    def test_only_elapsed_micros_is_normalized(self):
        self.assertEqual(r.normalized({'x':[{'elapsed_micros':3,'status':'partial','elapsed_ms':4}]}),{'x':[{'status':'partial','elapsed_ms':4}]})

@unittest.skipUnless((r.ROOT/r.PACKET/'actual/receipt.json.gz').exists(),'runtime not archived')
class Runtime(unittest.TestCase):
    def mutant(self,name,change):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp)
            for x in (r.ROOT/r.PACKET/'actual').rglob('*'):
                if x.is_file():
                    target=p/x.relative_to(r.ROOT/r.PACKET/'actual');target.parent.mkdir(parents=True,exist_ok=True);target.symlink_to(x)
            target=p/name;d=r.read(target);change(d);target.unlink();r.compress(target,json.dumps(d).encode())
            with self.assertRaises(ValueError):r.inspect_runtime(p,r.ROOT/r.PACKET,True)
    def test_actual_default_measured_and_failure_denominators(self):
        d=r.inspect_runtime(r.ROOT/r.PACKET/'actual',r.ROOT/r.PACKET,True)
        self.assertEqual((d['sessions'],d['successful_sessions'],d['expected_provider_startup_refusals']),(6,3,3))
    def test_forced_cleanup_not_promoted(self):self.mutant('batch/session.json.gz',lambda d:d.update(term_sent=True))
    def test_scalar_retry_not_hidden(self):self.mutant('outcomes.json.gz',lambda d:d[1].update(startup_embedding_HTTP=66))
    def test_source_partial_not_promoted(self):
        self.mutant('batch/source-response.json.gz',lambda d:d['raw_response'].update(result={'status':'complete'}))
    def test_independent_approval_not_invented(self):self.mutant('SCOPE.json.gz',lambda d:d.update(independent_approved=1))


@unittest.skipUnless((r.ROOT/r.PACKET/'SUMMARY.json').exists(),'whole native evidence not sealed')
class SourceBound(unittest.TestCase):
    def test_current_full_source_and_all_attempts_verify(self):
        self.assertEqual(r.validate(),[])
        summary=r.read(r.ROOT/r.PACKET/'SUMMARY.json')
        self.assertEqual(summary['native']['changed_Go_files'],10)
        self.assertEqual(summary['all_CKS_sessions'],16)
    def test_resealed_summary_cannot_claim_independent_completion(self):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp)
            for x in (r.ROOT/r.PACKET).rglob('*'):
                if x.is_file() and x.name!='manifest.json':
                    f=p/x.relative_to(r.ROOT/r.PACKET);f.parent.mkdir(parents=True,exist_ok=True);f.symlink_to(x)
            f=p/'SUMMARY.json';d=r.read(f);d.update(formal_completed=9,independent_approved=1);f.unlink();r.write(f,d);r.seal(p)
            self.assertTrue(r.validate(packet=p))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
