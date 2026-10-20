"""Source-context, exact AST ownership, retained provenance and refusal controls."""
import base64
import copy
import gzip
import json
from pathlib import Path
import shutil
import tempfile
import unittest
import wi3_consumer_matching as g


class SourceAndAdapter(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.p=g.ROOT/g.PACKET;cls.values=g.derive(g.ROOT,cls.p)
        source,cls.dataset,_,cls.pins=g.read_bindings(cls.p)
        cls.manifest={(r['origin_id'],r['path']):r for r in source['files']}
        cls.files={(r['origin_id'],r['path']):base64.b64decode(r['base64']) for r in json.loads(gzip.decompress((cls.p/'runtime/captured-source-files.json.gz').read_bytes()))}
        cls.decls={r['path']:r['functions'] for r in json.loads(gzip.decompress((cls.p/'source-audit/stdout.json.gz').read_bytes()))}
        cls.rows=g.f.read(cls.p/'runtime/primary-results.json')
        cls.doc_candidate=next(r for r in cls.rows if r['case_id']=='RQ-006')['raw_response']['result']['structuredContent']['candidates'][1]
    def adapt(self,candidate,decls=None):return g.adapt_candidate(candidate,self.values['overlay']['locations'],self.files,self.manifest,self.pins,self.dataset,self.decls if decls is None else decls)
    def test_source_only_two_refinements(self):
        overlay=self.values['overlay'];self.assertEqual(overlay['labels'],dict(relevant=25,irrelevant=364,unknown=11))
        changed=[(d['case_id'],u['location_id']) for d in overlay['gold'] for u in d['universe'] if u.get('previous_label')]
        self.assertEqual(changed,[('RQ-007','RQ-008'),('RQ-008','RQ-007')]);self.assertTrue(all(not u['root_cause_established'] for d in overlay['gold'] for u in d['universe']))
    def test_old_source_packet_not_relabelled(self):self.assertEqual(g.f.read(g.ROOT/g.old.PACKET/'SUMMARY.json')['labels'],dict(relevant=23,irrelevant=364,unknown=13))
    def test_unknown_meaning_and_support_are_distinct(self):
        self.assertEqual(len(self.values['overlay']['unknowns']),11);self.assertTrue(all('not negative' in u['meaning'] for u in self.values['overlay']['unknowns']))
        self.assertIsNone(self.values['overlay']['independent_review'])
    def test_all20_partial_retained_in_denominator(self):
        self.assertEqual(len(self.values['cases']),20);self.assertTrue(all(c['status']=='partial' and c['primary_partial'] and c['stop_reasons'] for c in self.values['cases']))
    def test_actual_public_symptom_requests_no_source_hints(self):
        expected=g.f.read(g.ROOT/g.old.PACKET/'public/wire-requests.json')['requests']
        self.assertEqual([r['arguments'] for r in self.rows],[r['arguments'] for r in expected]);self.assertTrue(all(not {'symbol','file','expand'}.intersection(r['arguments']) for r in self.rows))
    def test_657_retained_anchor_instances_verified(self):
        self.assertEqual(self.values['summary']['retained_anchor_instances_verified'],657);self.assertEqual(self.values['summary']['captured_source_files'],46)
    def test_strict0_and_supplemental2_separate(self):
        self.assertEqual(self.values['summary']['strict_prior_adapter_mapped'],0);self.assertEqual(self.values['summary']['supplemental_AST_doc_envelopes'],2)
    def test_exact_associated_doc_maps_without_name(self):
        c=copy.deepcopy(self.doc_candidate);c['symbol']='fabricated name ignored';self.assertEqual(self.adapt(c)['matching']['status'],'mapped')
    def test_without_AST_ownership_doc_is_unknown(self):self.assertEqual(self.adapt(self.doc_candidate,{})['matching']['status'],'non_equivalent_source_span_unknown')
    def test_arbitrary_prefix_cannot_acquire_function_identity(self):
        c=copy.deepcopy(self.doc_candidate);ref=c['retained_citation'];ref['start_line']=1;c['citation']['start_line']=1
        raw=self.files['repo',ref['file']];_,_,span=g.line_span(raw,1,ref['end_line']);ref['content_sha256']=g.f.sha(span)
        self.assertIsNone(self.adapt(c)['matching']['location_key'])
    def test_same_file_other_function_is_outside(self):
        row=next(r for r in self.rows if r['case_id']=='RQ-011');candidate=row['raw_response']['result']['structuredContent']['candidates'][0]
        self.assertEqual(self.adapt(candidate)['matching']['status'],'outside_selected_universe_unknown')
    def test_outside66_not_false_positives_or_negatives(self):
        self.assertEqual(self.values['summary']['candidate_matching'],dict(outside_selected_universe_unknown=66,mapped=2))
        self.assertTrue(all(r['AI_source_label'] is None for c in self.values['cases'] for r in c['raw_candidates'] if r['matching']['location_key']is None));self.assertIsNone(self.values['summary']['current_scores'])
    def test_raw_ranks_preserved_without_refill(self):
        for c in self.values['cases']:self.assertEqual([r['raw_rank'] for r in c['raw_candidates']],list(range(1,c['raw_count']+1)));self.assertLessEqual(c['raw_count'],10)
    def test_scope_refusals_even_when_source_is_same(self):
        for key in ('project_id','dataset_id','snapshot_id','source_mode','base_commit','commit_hash','origin_id'):
            c=copy.deepcopy(self.doc_candidate);c['retained_citation'][key]='wrong'
            with self.assertRaises(ValueError):self.adapt(c)
    def test_content_hash_refusal(self):
        c=copy.deepcopy(self.doc_candidate);c['retained_citation']['content_sha256']='0'*64
        with self.assertRaises(ValueError):self.adapt(c)
    def test_legacy_v2_disagreement_refused(self):
        c=copy.deepcopy(self.doc_candidate);c['citation']['start_line']+=1
        with self.assertRaises(ValueError):self.adapt(c)
    def test_utf8_line_coordinates_and_CRLF_exact_bytes(self):
        raw='// 한글\r\nfunc F(){}\n'.encode();a,b,span=g.line_span(raw,2,2);self.assertEqual(a,len('// 한글\r\n'.encode()));self.assertEqual(b,len(raw));self.assertEqual(span,b'func F(){}\n')
    def test_invalid_line_types_refused(self):
        for first,last in [(True,1),(0,1),(2,1),(1,3)]:
            with self.assertRaises(ValueError):g.line_span(b'a\n',first,last)
    def test_unsupported_and_error_signals_not_imputed(self):
        counts=self.values['summary']['signal_statuses'];self.assertEqual(counts['security_tag:unsupported'],68);self.assertEqual(counts['dataflow_on_path:unsupported'],68);self.assertGreater(counts['impact_category:error'],0)
    def test_actual_three_refusals_and_normal_EOF(self):
        self.assertEqual(g.wire_errors(self.p),[]);self.assertEqual(g.f.read(self.p/'runtime/session.json')['exit'],0)
    def test_no_agent_role_or_independent_quality_claim(self):
        self.assertEqual(self.values['summary']['installed_agent_role_runs'],0);self.assertEqual(self.values['summary']['independent_approved'],0);self.assertTrue(all(c['quality_score']is None for c in self.values['cases']))


class ResealedMutants(unittest.TestCase):
    def mutant(self,name,change,packed=False):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp)/'packet';shutil.copytree(g.ROOT/g.PACKET,p);path=p/name;v=json.loads(gzip.decompress(path.read_bytes())) if packed else g.f.read(path);change(v)
            if packed:path.write_bytes(gzip.compress(g.f.canonical(v),mtime=0))
            else:path.write_text(json.dumps(v,ensure_ascii=False))
            m=g.f.read(p/'manifest.json');m['artifacts']={x.relative_to(p).as_posix():g.f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'};(p/'manifest.json').write_text(json.dumps(m));self.assertTrue(g.validate(packet=p),name)
    def test_current_packet(self):self.assertEqual(g.validate(),[]);self.assertEqual(g.current_indices(),[])
    def test_source_unknown_erased(self):self.mutant('private/overlay.json.gz',lambda v:v['unknowns'].clear(),True)
    def test_unknown_unit_flipped(self):self.mutant('private/overlay.json.gz',lambda v:next(u for d in v['gold'] for u in d['universe'] if u['label']=='unknown').update(label='irrelevant'),True)
    def test_partial_promoted(self):self.mutant('SUMMARY.json',lambda v:v.update(statuses=dict(response=20)))
    def test_quality_score_invented(self):self.mutant('SUMMARY.json',lambda v:v.update(current_scores=dict(recall_at_10=1)))
    def test_missing_planned_case(self):self.mutant('runtime/primary-results.json',lambda v:v.pop())
    def test_primary_hint_injection(self):self.mutant('runtime/primary-results.json',lambda v:v[0]['arguments'].update(symbol='DecodeRLP'))
    def test_provenance_content_tamper(self):self.mutant('runtime/primary-results.json',lambda v:v[0]['raw_response']['result']['structuredContent']['candidates'][0]['retained_citation'].update(content_sha256='0'*64))
    def test_source_AST_doc_prefix_tamper(self):self.mutant('source-audit/stdout.json.gz',lambda v:v[0]['functions'][0].update(doc_start=0),True)
    def test_review_debt_erased(self):self.mutant('REVIEW-LEDGER.json',lambda v:v['unlabelled_or_unmapped_slots'].clear())
    def test_model_digest_tamper(self):self.mutant('runtime/model-before.json',lambda v:v['model'].update(digest='0'*64))
    def test_retained_inventory_tamper(self):self.mutant('runtime/retained-after.json',lambda v:v.pop(next(iter(v))))
    def test_reviewer_invention(self):self.mutant('WORKLIST.json',lambda v:v.update(reviewer_assignment='AI'))
    def test_matching_promoted(self):self.mutant('matching-contract.json',lambda v:v.update(formal_scoring_allowed=True))
    def test_independent_acceptance_invented(self):self.mutant('runtime/receipt.json',lambda v:v.update(independent_approved=20))
    def test_role_run_invented(self):self.mutant('runtime/receipt.json',lambda v:v.update(installed_agent_role_runs=1))


if __name__=='__main__':unittest.main()
