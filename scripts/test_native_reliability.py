"""Replay source-bound native/actual-consumer evidence, rejecting false acceptance."""
import json
import tempfile
import unittest
from pathlib import Path
import native_reliability as n


class Contract(unittest.TestCase):
    def test_only_timing_fields_are_normalized(self):
        value={'elapsed_micros':4,'citations':[{'commit':'abc','score':.5,'elapsed_ms':2}]}
        self.assertEqual(n.normalize_search(value),{'citations':[{'commit':'abc','score':.5}]})

    def test_scope_keeps_original_limits_and_unassigned_acceptance(self):
        s=n.runtime_scope('0'*64)
        self.assertEqual(s['cache_entries'],[0,32])
        self.assertEqual(s['planned_tools'],46)
        self.assertEqual(s['formal_completed'],0)
        self.assertEqual(s['limits']['timeout_ms'],1000)


@unittest.skipUnless((n.ROOT/n.PACKET/'manifest.json').is_file(),'actual capture not prepared')
class Captured(unittest.TestCase):
    def mutant(self,name,change):
        with tempfile.TemporaryDirectory()as tmp:
            p=Path(tmp)/'packet';p.mkdir()
            for source in (n.ROOT/n.PACKET).rglob('*'):
                if source.is_file()and source.name!='manifest.json':
                    target=p/source.relative_to(n.ROOT/n.PACKET);target.parent.mkdir(parents=True,exist_ok=True);target.symlink_to(source)
            target=p/name;data=n.read(target);change(data);target.unlink()
            if target.suffix=='.gz':n.compressed(target,json.dumps(data).encode())
            else:n.write(target,data)
            n.seal(p)
            self.assertTrue(n.validate(packet=p))

    def test_actual_current_native_and_consumer_evidence(self):
        self.assertEqual(n.validate(),[])
        data=n.derive()['summary']
        self.assertEqual(data['runtime']['selected_receive_owner_corrections'],16)
        self.assertEqual(data['runtime']['cache_on_query_embedding_HTTP_calls'],1)
        self.assertEqual(data['formal_completed'],0)

    def test_resealed_false_independent_acceptance_rejected(self):
        self.mutant('SUMMARY.json',lambda d:d.update(independent_approved=1))

    def test_resealed_query_cache_effect_invention_rejected(self):
        self.mutant('actual/search-counts.json.gz',lambda d:d[0].update(embedding_HTTP_calls=0))

    def test_resealed_runtime_pin_change_rejected(self):
        self.mutant('RUNTIME-SCOPE.json',lambda d:d['pins'].update(commit='0'*40))


if __name__=='__main__':unittest.main()
