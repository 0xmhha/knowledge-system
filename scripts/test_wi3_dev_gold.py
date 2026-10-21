"""Source adjudication counterexamples, rank/scope controls and resealed mutants."""
import copy
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import wi3_dev_gold as g
import evidence_revision


class DraftAndMatching(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.p=g.ROOT/g.PACKET;cls.data=g.derive();cls.gold={d['case_id']:d for d in cls.data['gold']}
        cls.locations=cls.data['locations'];cls.pins=g.f.read(cls.p/'proposed-reader-pins.json')['pins']
    def labels(self,identity,label):return [u['location_id'] for u in self.gold[identity]['universe'] if u['label']==label]
    def hit(self,index=0):return dict(self.pins,**{k:v for k,v in self.locations[index]['identity'].items() if k not in ('project','commit','source_mode')},unit_kind='function')
    def test_complete_partition_not_repository_gold(self):
        self.assertEqual(len(self.gold),20)
        for d in self.gold.values():
            self.assertEqual(len(d['universe']),20);self.assertFalse(d['semantic_gold_complete']);self.assertIsNone(d['repository_wide_labels'])
            self.assertEqual(sorted(u['location_key'] for u in d['universe']),sorted(k for values in d['answer_sets'].values() for k in values))
            self.assertTrue(all(d['empty_set_rationales'].get(k) for k,v in d['answer_sets'].items() if not v))
    def test_manually_reviewed_direct20(self):
        for identity in self.gold:self.assertIn(identity,self.labels(identity,'relevant'))
    def test_supporting_context_is_not_family_propagation(self):
        self.assertEqual(self.labels('RQ-005','relevant'),['RQ-005','RQ-006'])
        self.assertEqual(self.labels('RQ-012','relevant'),['RQ-012','RQ-014'])
        self.assertEqual(self.labels('RQ-013','relevant'),['RQ-013','RQ-014'])
        self.assertEqual(self.labels('RQ-003','relevant'),['RQ-003'])
    def test_same_group_unknown_not_positive(self):
        for a,b in [('RQ-003','RQ-004'),('RQ-007','RQ-008'),('RQ-009','RQ-011')]:self.assertIn(b,self.labels(a,'unknown'))
    def test_codec_names_do_not_make_same_relevance(self):
        self.assertIn('RQ-016',self.labels('RQ-001','irrelevant'));self.assertIn('RQ-001',self.labels('RQ-016','irrelevant'))
    def test_comparator_and_transitive_effects_unknown(self):
        self.assertIn('RQ-012',self.labels('RQ-014','unknown'));self.assertIn('RQ-020',self.labels('RQ-018','unknown'))
    def test_scope_and_hash_exact_match(self):self.assertEqual(g.match_hit(self.hit(),self.locations,self.pins)['status'],'mapped')
    def test_each_wrong_scope_rejected(self):
        for key in self.pins:
            hit=self.hit();hit[key]='wrong';self.assertEqual(g.match_hit(hit,self.locations,self.pins)['status'],'refused_wrong_'+key)
    def test_name_only_is_unknown(self):self.assertEqual(g.match_hit(dict(name=self.locations[0]['name']),self.locations,self.pins)['status'],'unknown_missing_scope')
    def test_missing_version_not_matched(self):
        hit=self.hit();hit.pop('commit');self.assertIsNone(g.match_hit(hit,self.locations,self.pins)['location_key'])
    def test_wrong_file_or_function_hash_unknown(self):
        for k in ('path','file_sha256','function_sha256'):
            hit=self.hit();hit[k]='wrong';self.assertEqual(g.match_hit(hit,self.locations,self.pins)['status'],'outside_or_unresolved_unknown')
    def test_declared_span_and_not_substring(self):
        hit=self.hit();span=self.locations[0]['equivalence_spans'][0];hit.update(span,unit_kind='span');self.assertEqual(g.match_hit(hit,self.locations,self.pins)['status'],'mapped')
        hit['start_byte']+=1;self.assertIsNone(g.match_hit(hit,self.locations,self.pins)['location_key'])
    def test_duplicate_slots_not_refilled(self):
        result=g.project_top10([self.hit()]*10+[self.hit(1)],self.locations,self.pins)
        self.assertEqual(len(result['raw_positions']),10);self.assertEqual(len(result['unique_mapped_keys']),1);self.assertEqual(sum(r['duplicate'] for r in result['raw_positions']),9)
    def test_unknown_slot_does_not_pull_rank11(self):
        result=g.project_top10([{}]*10+[self.hit()],self.locations,self.pins);self.assertEqual(result['unique_mapped_keys'],[]);self.assertIsNone(result['score'])
    def test_family_and_exposure_not_holdout(self):
        self.assertTrue(all(d['exposure']['fresh_holdout'] is False and d['independent_family'] is None for d in self.gold.values()));self.assertEqual(self.data['summary']['proposed_groups'],15)
    def test_public_has_no_source_hints(self):
        public=json.dumps(g.f.read(self.p/'public/DEV-inputs.json'),ensure_ascii=False)
        for l in self.locations:self.assertNotIn(l['identity']['path'],public);self.assertNotIn(l['name'],public)
    def test_no_runtime_or_scores(self):
        self.assertEqual(self.data['summary']['new_retrieval_calls'],0);self.assertTrue(all(d['actual_retrieval_receipt'] is None and d['current_scores'] is None for d in self.gold.values()))


class Mutations(unittest.TestCase):
    def mutant(self,name,change):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp)/'packet';shutil.copytree(g.ROOT/g.PACKET,p);path=p/name;value=g.f.read(path);change(value);path.write_text(json.dumps(value,ensure_ascii=False));m=g.f.read(p/'manifest.json');m['artifacts']={x.relative_to(p).as_posix():g.f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'};(p/'manifest.json').write_text(json.dumps(m));self.assertTrue(g.validate(packet=p),name)
    def test_current_packet(self):self.assertEqual(g.validate(),[]);self.assertEqual(g.current_indices(),[])
    def test_missing_universe_member(self):self.mutant('private/AI-gold.json',lambda v:v[0]['universe'].pop())
    def test_unknown_flipped_to_negative(self):self.mutant('private/AI-gold.json',lambda v:next(u for u in v[2]['universe'] if u['label']=='unknown').update(label='irrelevant'))
    def test_empty_rationale_erased(self):self.mutant('private/AI-gold.json',lambda v:v[0]['empty_set_rationales'].clear())
    def test_answer_set_member_erased(self):self.mutant('private/AI-gold.json',lambda v:v[0]['answer_sets']['relevant_locations'].clear())
    def test_root_cause_fabrication(self):self.mutant('private/AI-gold.json',lambda v:v[0]['universe'][0].update(root_cause_established=True))
    def test_repository_wide_claim(self):self.mutant('SUMMARY.json',lambda v:v.update(repository_wide_gold_complete=True))
    def test_independent_approval_fabricated(self):self.mutant('SUMMARY.json',lambda v:v.update(independent_approved=20))
    def test_family_split_fabricated(self):self.mutant('exposure-and-families.json',lambda v:v.update(independent_families=20))
    def test_unmarked_exposure(self):self.mutant('exposure-and-families.json',lambda v:v.update(all_exposed=False))
    def test_final_relabel(self):self.mutant('public/DEV-inputs.json',lambda v:v[0].update(split='FINAL'))
    def test_public_label_leak(self):self.mutant('public/DEV-inputs.json',lambda v:v[0].update(gold_locations=[]))
    def test_runtime_fabrication(self):self.mutant('proposed-reader-pins.json',lambda v:v.update(actual_search_receipt=dict(exit=0)))
    def test_quality_score_fabrication(self):self.mutant('SUMMARY.json',lambda v:v.update(current_scores=dict(recall_at_10=1)))
    def test_dedup_before_cut_without_approval(self):self.mutant('matching-contract.json',lambda v:v.update(top10='dedup and refill from full corpus'))
    def test_reviewer_invention(self):self.mutant('WORKLIST.json',lambda v:v.update(reviewer_assignment='AI'))
    def test_formal_completion_fabrication(self):self.mutant('WORKLIST.json',lambda v:v.update(formal_completed=1))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
