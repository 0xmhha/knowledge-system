"""Check semantic separation and reject tempting reference/matching shortcuts."""
import copy,gzip,json,shutil,tempfile,unittest
from pathlib import Path
import wi1_select_review as g


class SelectReference(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.packet=Path(self.temp.name)/'packet';shutil.copytree(g.ROOT/g.PACKET,self.packet)

    def mutate_observation(self,edit):
        p=self.packet/'observations.json.gz';d=g.matching.old.packed(p);edit(d)
        p.write_bytes(gzip.compress(json.dumps(d).encode(),mtime=0))

    def test_current_reference(self):
        s=g.derive(packet=self.packet)['summary'];self.assertEqual(s['lexical_disagreements'],0)
        self.assertEqual(s['unmatched_occurrence_phases'],53);self.assertEqual(s['unmatched_blocks'],3)
        self.assertEqual(s['value_labels'],dict(unknown=1427,positive=212,negative=15))

    def test_backward_pairs_are_killed(self):
        d=g.derive(packet=self.packet)
        rows=[r for r in d['pairs']if r['before']['structural_reaches']is True and r['after']['structural_reaches']is False]
        self.assertEqual({r['after']['use']for r in rows},{'u:o@18473','u:o@18569'})
        self.assertTrue(all(r['case_id']=='SRC-027' and r['after']['label']=='unknown'for r in rows))

    def test_no_new_case_or_candidate_run(self):
        d=g.derive(packet=self.packet)['summary'];self.assertEqual((d['functions'],d['pairs']),(30,1654))
        for key in ('new_actual_consumer_calls','new_independent_samples','independent_approved','formal_completed'):
            self.assertEqual(d[key],0)
        self.assertIsNone(d['quality_score']);self.assertFalse(d['execution_allowed'])

    def test_occurrence_deletion_rejected(self):
        self.mutate_observation(lambda d:d[0]['occurrences'].pop())
        with self.assertRaises(ValueError):g.derive(packet=self.packet)

    def test_source_change_rejected(self):
        self.mutate_observation(lambda d:d[0].update(function_sha256='0'*64))
        with self.assertRaises(ValueError):g.derive(packet=self.packet)

    def test_numeric_owner_shortcut_rejected(self):
        def edit(d):
            o=next(o for x in d for o in x['occurrences']if o['mapping_reason']=='selected_receive_LHS_exact_AST_case_identity')
            o['owner']='entry'
        self.mutate_observation(edit)
        with self.assertRaises((ValueError,KeyError)):g.derive(packet=self.packet)

    def test_nonambiguous_owner_rewrite_rejected(self):
        def edit(d):
            o=next(o for o in d[0]['occurrences']if o['owner']=='entry');o['mapping_reason']='selected_receive_LHS_exact_AST_case_identity'
        self.mutate_observation(edit)
        with self.assertRaises(ValueError):g.derive(packet=self.packet)

    def test_type_promotion_rejected(self):
        def edit(d):
            o=next(x for x in d if x['name']=='newWorkLoopOrigin');next(b for b in o['bindings']if b['id']=='v@18380')['value_kind']='scalar'
        self.mutate_observation(edit)
        with self.assertRaises(ValueError):g.derive(packet=self.packet)

    def test_runtime_value_fabrication_rejected(self):
        (self.packet/'runtime-witness-output.json').write_text(json.dumps(dict(before=[3,5],after=[5,5])))
        with self.assertRaises(ValueError):g.derive(packet=self.packet)

    def test_control_missing_second_lhs_stays_unknown(self):
        p=self.packet/'controls-observations.json.gz';d=g.matching.old.packed(p)
        o=next(x for x in d if x['name']=='TwoValue');b=next(b for b in o['bindings']if b['name']=='ok')
        a=next(x for x in o['occurrences']if x['binding']==b['id'] and x['role']=='definition');a['owner']='entry'
        p.write_bytes(gzip.compress(json.dumps(d).encode(),mtime=0))
        with self.assertRaises(ValueError):g.derive(packet=self.packet)


if __name__=='__main__':unittest.main()
