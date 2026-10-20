"""Manual relation vs semantics, provenance/type boundaries and completion controls."""
import copy,gzip,json,shutil,tempfile,unittest
from pathlib import Path
import wisp_dev_gold as g


class SourceAndMetadata(unittest.TestCase):
    @classmethod
    def setUpClass(cls):cls.data=g.derive();cls.gold={r['case_id']:r for r in cls.data['gold']}
    def test_complete12x46_selector_partition(self):
        self.assertEqual(len(self.gold),12)
        for r in self.gold.values():
            self.assertEqual(len(r['universe']),46)
            self.assertEqual(sorted(u['annotation_id'] for u in r['universe']),sorted(k for v in r['answer_sets'].values() for k in v))
            self.assertTrue(all(r['empty_set_rationales'].get(k) for k,v in r['answer_sets'].items() if not v))
            self.assertTrue(all(u['semantic_applicability']=='unknown' for u in r['universe']))
    def test_policy5_security_proposals4_not_actual_ingest(self):
        self.assertEqual(sum(len(r['policy_only_expected']) for r in self.gold.values()),5)
        self.assertEqual(sum(len(r['proposed_combined_expected']) for r in self.gold.values()),9)
        self.assertFalse(self.data['summary']['proposed_security_applied'])
        self.assertTrue(all(r['overlay_ingest_receipt']is None for r in self.gold.values()))
    def test_relative_untagged_not_safety_or_no_semantic_policy(self):
        self.assertEqual(sum(r['policy_only_relative_untagged'] for r in self.gold.values()),8)
        self.assertEqual(sum(r['proposed_combined_relative_untagged'] for r in self.gold.values()),4)
        self.assertTrue(all(r['safety_verdict']=='unknown' for r in self.gold.values()))
    def test_description_mention_does_not_create_governs(self):
        for name in ('SPC-009','SPC-011','SPC-008'):
            self.assertEqual(self.gold[name]['policy_only_expected'],[])
        self.assertIn('GetMinterCount',next(e for e in self.data['entries'] if e['id']=='A4.gov_minter.mint_proposal_allowance')['description'])
    def test_exact_wire_kind_enum(self):
        self.assertEqual(self.gold['SPC-001']['policy_only_expected'][0]['kind'],'Policy')
        self.assertEqual(self.gold['SPC-005']['proposed_combined_expected'][0]['kind'],'SecurityPattern')
    def test_modern_known_empty_differs_from_unknown(self):
        a=self.gold['SPC-001']['policy_only_expected'][0]
        self.assertEqual(a['severity'],g.known(''));self.assertEqual(a['modification_hint'],g.known(''))
        self.assertNotEqual(a['severity'],g.unknown())
    def test_security_low_manual_not_confirmed_vulnerability(self):
        for name in ('SPC-005','SPC-006','SPC-007','SPC-008'):
            a=self.gold[name]['proposed_combined_expected'][0]
            self.assertEqual(a['severity'],g.known('low'));self.assertFalse(a['automatic_rule_candidate']);self.assertFalse(a['vulnerability_or_compliance_approved'])
    def test_all_actual_original_provenance_fields_remain_unknown(self):
        for r in self.gold.values():
            for a in r['proposed_combined_expected']:
                self.assertEqual(a['provenance']['identity_trust'],'not_authenticated')
                for field in g.UNKNOWN_PROVENANCE:self.assertEqual(a['provenance'][field],g.unknown())
    def test_authoring_yaml_row_hash_is_not_runtime_source_authentication(self):
        e=next(e for e in self.data['entries'] if e['kind']=='policy');s=e['authoring_source'];raw=(g.ROOT/s['path']).read_bytes()
        self.assertEqual(g.f.sha(raw),s['sha256']);self.assertEqual(g.f.sha(raw[s['start_byte']:s['end_byte_exclusive']]),s['span_sha256'])
        self.assertIsNone(e['semantic_approval'])
    def test_all_source_bodies_and_exact_project_commit_bound(self):
        for r in self.gold.values():
            s=r['source'];self.assertEqual(g.f.sha(s['body'].encode()),s['span_sha256']);self.assertEqual(s['commit'],g.f.SOURCE_COMMIT)
    def test_legacy_merged_text_not_guessed_severity_or_hint(self):
        r=g.legacy_expected('high severity\nfix: rotate keys')
        self.assertEqual(r['explanation'],g.known('high severity\nfix: rotate keys'))
        self.assertEqual(r['severity'],g.unknown());self.assertEqual(r['modification_hint'],g.unknown())
    def test_public_no_annotation_ids_or_security_answers(self):
        for r in self.data['public']:
            self.assertTrue({'answer_sets','policies','security','proposed_combined_expected'}.isdisjoint(r))
            self.assertEqual(r['split'],'DEV');self.assertFalse(r['contains_gold'])
            self.assertTrue((g.ROOT/g.PACKET/'public'/r['source_excerpt']).is_file())
    def test_controls16_are_proposals_not_actual_refusal_scores(self):
        self.assertEqual(len(self.data['controls']),16);self.assertIsNone(self.data['summary']['actual_scores'])
        self.assertEqual(self.data['summary']['actual_symbol_queries'],0);self.assertEqual(self.data['summary']['actual_automatic_queries'],0)
    def test_final_drift_uses_actual_snapshot_changed_status(self):
        r=next(r for r in self.data['controls'] if r['id']=='SP-CTRL-10');self.assertEqual(r['expected'],'snapshot_changed')
    def test_project_store_binding_not_invented_wire_field(self):
        self.assertFalse(g.review_ledger(self.data)['wire_project_dataset_fields_present'])
        self.assertIn('routed store/session',g.contract()['identity'])
    def test_decoder_caps_and_ACL_are_separate_from_symbol_authorization(self):
        self.assertIn('HTTP403_before_tool',[r['expected'] for r in self.data['controls']])
        self.assertIn('not per-symbol/project authorization',g.contract()['ACL'])
    def test_source_file_hash_not_used_as_blob_pin(self):
        self.assertTrue(all(r['blob_hash_adapter']is None for r in self.gold.values()))
        self.assertIn('Full-file SHA',g.contract()['source'].replace('original full-file SHA','Full-file SHA'))
    def test_no_family_independence_or_formal_approval(self):
        self.assertEqual(self.data['summary']['independent_families'],0);self.assertFalse(self.data['summary']['execution_allowed'])


class ResealedMutations(unittest.TestCase):
    def mutant(self,name,change,compressed=False):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp)/'packet';shutil.copytree(g.ROOT/g.PACKET,p);x=p/name;raw=x.read_bytes();v=json.loads(gzip.decompress(raw) if compressed else raw);old=copy.deepcopy(v);change(v);self.assertNotEqual(v,old)
            raw=g.f.canonical(v);x.write_bytes(gzip.compress(raw,mtime=0) if compressed else raw)
            m=g.f.read(p/'manifest.json');m['artifacts']={x.relative_to(p).as_posix():g.f.sha(x.read_bytes()) for x in p.rglob('*') if x.is_file() and x!=p/'manifest.json'};(p/'manifest.json').write_text(json.dumps(m));self.assertTrue(g.validate(packet=p))
    def test_current_packet_and_live_indices(self):self.assertEqual(g.validate(),[]);self.assertEqual(g.current_indices(),[])
    def test_selector_absence_promoted_to_semantic_negative(self):self.mutant('private/AI-gold.json.gz',lambda v:v[0]['universe'][0].update(semantic_applicability='not_applicable'),True)
    def test_operator_tag_promoted_to_vulnerability(self):self.mutant('private/AI-gold.json.gz',lambda v:v[4]['proposed_combined_expected'][0].update(vulnerability_or_compliance_approved=True),True)
    def test_runtime_YAML_hash_fabricated(self):self.mutant('private/AI-gold.json.gz',lambda v:v[0]['policy_only_expected'][0]['provenance'].update(source_sha256=g.known('a'*64)),True)
    def test_reviewer_fabricated_from_authoring_source(self):self.mutant('private/AI-gold.json.gz',lambda v:v[0]['policy_only_expected'][0]['provenance'].update(reviewer=g.known('AI')),True)
    def test_modern_empty_replaced_with_unknown(self):self.mutant('private/AI-gold.json.gz',lambda v:v[0]['policy_only_expected'][0].update(severity=g.unknown()),True)
    def test_security_proposal_applied_flag_fabricated(self):self.mutant('SUMMARY.json',lambda v:v.update(proposed_security_applied=True))
    def test_actual_query_receipt_fabricated(self):self.mutant('private/AI-gold.json.gz',lambda v:v[0].update(actual_manual_query={'status':'annotations_found'}),True)
    def test_no_annotations_is_safety_false_claim(self):self.mutant('private/AI-gold.json.gz',lambda v:v[8].update(safety_verdict='safe'),True)
    def test_missing_selector_unit(self):self.mutant('private/AI-gold.json.gz',lambda v:v[0]['universe'].pop(),True)
    def test_control_outcome_changed(self):self.mutant('private/controls.json',lambda v:v[9].update(expected='annotations_found'))
    def test_private_gold_in_public(self):self.mutant('public/DEV-inputs.json',lambda v:v[0].update(expected_annotations=[]))
    def test_automatic_overwrites_manual(self):self.mutant('matching-contract.json',lambda v:v.update(wire='automatic rule findings stand in for manual metadata'))
    def test_graph_hash_promoted_to_authentication(self):self.mutant('matching-contract.json',lambda v:v.update(provenance='metadata SHA authenticates reviewer'))
    def test_deployed_ACL_receipts_invented(self):self.mutant('REVIEW-LEDGER.json',lambda v:v.update(missing=[]))
    def test_independent_symbols_invented(self):self.mutant('SUMMARY.json',lambda v:v.update(independent_families=12))
    def test_formal_acceptance_invented(self):self.mutant('WORKLIST.json',lambda v:v.update(formal_completed=1))
    def test_real_name_requirement_cannot_return(self):self.mutant('reviewer-identity-policy.json',lambda v:v.update(real_name_required=True))


if __name__=='__main__':unittest.main()
