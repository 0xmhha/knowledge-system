"""Actual envelope matching, exact retained discovery, partial and unknown cases."""
import copy
import json
import sqlite3
import tempfile
import unittest
from pathlib import Path

import wi6_pair_contract as c


def wire(value):
    return dict(jsonrpc='2.0', id=1, result=dict(structuredContent=value))


class EnvelopeTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.data = c.gold.derive()
        cls.locations = cls.data['locations']
        cls.draft = next(d for d in cls.data['gold'] if d['seed_excluded_recall_defined'])
        cls.location = next(l for l in cls.locations if l['key'] == cls.draft['seed_key'])
        cls.pins = dict(project_id='owned-project', dataset_id='owned-data', source_mode='committed',
                        commit=cls.location['identity']['commit'], graph_snapshot_sha256='a'*64)

    def result(self, location=None, node_id='node'):
        loc = location or self.location
        i = loc['identity']
        node = dict(i, node_id=node_id, start_line=1, bytes=i['end_byte_exclusive']-i['start_byte'])
        source = dict(node_id=node_id, file=i['path'], start_byte=i['start_byte'], start_line=1,
                      bytes=node['bytes'], sha256=i['function_sha256'], commit=self.pins['commit'],
                      snapshot_sha256=self.pins['graph_snapshot_sha256'])
        result = dict(status='analyzed_bounded', snapshot_sha256=self.pins['graph_snapshot_sha256'],
                      analysis=dict(source=source, status='analyzed_bounded', truncated=False))
        return result, {node_id: node}

    def view(self, arm, value, nodes):
        return c.view(arm, wire(value), nodes, self.pins, self.draft, self.locations)

    def test_structured_and_text_json_parsing(self):
        self.assertEqual(c.unpack(wire({'x':1})), ({'x':1}, 'structured'))
        self.assertEqual(c.unpack({'result':{'content':[{'text':'{"x":1}'}]}}), ({'x':1}, 'text_json'))

    def test_protocol_tool_and_malformed_errors_preserved(self):
        for raw, expected in [(None,'malformed_protocol'), ({'error':{}},'protocol_error'),
                              ({},'missing_tool_result'), ({'result':{'isError':True}},'tool_error'),
                              ({'result':{'content':[{'text':'not JSON'}]}},'malformed_tool_result')]:
            result = c.view('baseline',raw,{},self.pins,self.draft,self.locations)
            self.assertEqual(result['status'],expected);self.assertFalse(result['complete'])

    def test_exact_baseline_seed_is_mapped_but_excluded(self):
        result,nodes=self.result();v=self.view('baseline',result,nodes)
        self.assertTrue(v['complete']);self.assertTrue(v['raw_slots'][0]['seed'])
        self.assertFalse(v['raw_slots'][0]['primary_metric_eligible']);self.assertIsNone(v['quality_score'])

    def test_expanded_relevant_nonseed_uses_existing_source_gold(self):
        target=next(l for l in self.locations if l['key']==self.draft['seed_excluded_relevant'][0])
        result,nodes=self.result(target)
        value=dict(graph=dict(status='expanded_bounded',functions=[dict(result=result)],stop_reasons=[]))
        v=self.view('expansion',value,nodes)
        self.assertTrue(v['raw_slots'][0]['primary_metric_eligible'])
        self.assertEqual(v['raw_slots'][0]['AI_label'],'relevant');self.assertFalse(v['independent_gold'])

    def test_duplicates_kept_without_refill(self):
        result,nodes=self.result()
        v=self.view('expansion',dict(graph=dict(status='expanded_bounded',functions=[dict(result=result)]*2)),nodes)
        self.assertEqual(len(v['raw_slots']),2);self.assertTrue(v['raw_slots'][1]['duplicate'])

    def test_partial_mapped_identity_is_not_complete(self):
        result,nodes=self.result();result['status']='partial';result['analysis']['status']='partial'
        v=self.view('baseline',result,nodes)
        self.assertFalse(v['complete']);self.assertTrue(v['raw_slots'][0]['seed'])

    def test_truncated_analysis_is_not_complete(self):
        result,nodes=self.result();result['analysis']['truncated']=True
        self.assertFalse(self.view('baseline',result,nodes)['complete'])

    def test_missing_analysis_remains_a_slot(self):
        v=self.view('baseline',dict(status='budget_exhausted',analysis=None),{})
        self.assertEqual(len(v['raw_slots']),1);self.assertFalse(v['complete'])
        self.assertEqual(v['raw_slots'][0]['identity_check'],'missing_analysis')

    def test_missing_or_malformed_expansion_envelope_rejected(self):
        for value in ({},dict(graph={}),dict(graph={'functions':{}})):
            self.assertEqual(self.view('expansion',value,{})['status'],'malformed_analysis_envelope')

    def test_empty_or_budget_limited_expansion_not_complete(self):
        self.assertFalse(self.view('expansion',dict(graph=dict(status='expanded_bounded',functions=[])),{})['complete'])
        result,nodes=self.result()
        self.assertFalse(self.view('expansion',dict(graph=dict(status='budget_exhausted',functions=[dict(result=result)],stop_reasons=['deadline'])),nodes)['complete'])

    def test_wrong_source_snapshot_commit_and_coordinates_not_joined(self):
        for key in ('sha256','snapshot_sha256','commit','start_byte','bytes'):
            result,nodes=self.result();result['analysis']['source'][key]='wrong'
            v=self.view('baseline',result,nodes)
            self.assertFalse(v['complete']);self.assertIsNone(v['raw_slots'][0]['match']['location_key'])

    def test_missing_retained_identity_cannot_be_invented_from_wire(self):
        result,_=self.result();v=self.view('baseline',result,{})
        self.assertEqual(v['raw_slots'][0]['identity_check'],'missing_retained_node_evidence')

    def test_outside_gold_function_is_unknown_never_automatic_noise(self):
        result,nodes=self.result();node=nodes['node'];node['function_sha256']='b'*64
        result['analysis']['source']['sha256']='b'*64
        v=self.view('baseline',result,nodes)
        self.assertEqual(v['raw_slots'][0]['AI_label'],'outside_unknown')
        self.assertFalse(v['raw_slots'][0]['primary_metric_eligible'])

    def test_more_than_four_returned_functions_not_silently_capped(self):
        result,nodes=self.result()
        v=self.view('expansion',dict(graph=dict(status='expanded_bounded',functions=[dict(result=result)]*5)),nodes)
        self.assertEqual(v['status'],'returned_population_exceeds_declared_cap');self.assertEqual(v['returned_slots'],5)


class RetainedDiscovery(unittest.TestCase):
    def setUp(self):
        self.db=sqlite3.connect(':memory:');self.addCleanup(self.db.close)
        self.db.executescript('CREATE TABLE nodes(id TEXT,type TEXT,name TEXT,canonical_id TEXT,file_path TEXT,start_line INTEGER,end_line INTEGER,start_byte INTEGER,end_byte INTEGER); CREATE TABLE blobs(node_id TEXT,source BLOB);')
        self.blob=b'func F() {}';self.files={'a.go':'a'*64}
        self.db.execute('INSERT INTO nodes VALUES(?,?,?,?,?,?,?,?,?)',('id','Function','F','canonical','a.go',1,1,0,len(self.blob)))
        self.db.execute('INSERT INTO blobs VALUES(?,?)',('id',self.blob))
        self.source=dict(path='a.go',start_line=1,end_line=1,start_byte=0,end_byte_exclusive=len(self.blob),file_sha256='a'*64,function_sha256=c.sha(self.blob))

    def test_exact_node_and_blob_discovery(self):
        node,blob=c.select_seed(self.db,self.source,self.files)
        self.assertEqual(node['node_id'],'id');self.assertEqual(blob,self.blob)

    def test_same_name_wrong_coordinates_not_a_seed(self):
        with self.assertRaises(ValueError):c.select_seed(self.db,dict(self.source,start_byte=1),self.files)

    def test_changed_function_blob_or_file_hash_rejected(self):
        with self.assertRaises(ValueError):c.select_seed(self.db,dict(self.source,function_sha256='b'*64),self.files)
        with self.assertRaises(ValueError):c.select_seed(self.db,self.source,{'a.go':'b'*64})

    def test_ambiguous_coordinate_identity_rejected(self):
        self.db.execute('INSERT INTO nodes SELECT ?,type,name,canonical_id,file_path,start_line,end_line,start_byte,end_byte FROM nodes',('second',))
        with self.assertRaises(ValueError):c.select_seed(self.db,self.source,self.files)

    def test_missing_blob_or_nonfunction_identity_rejected(self):
        self.db.execute('DELETE FROM blobs')
        with self.assertRaises(ValueError):c.select_seed(self.db,self.source,self.files)

    def test_declared_canonical_separates_promoted_method_alias(self):
        self.db.execute('INSERT INTO nodes SELECT ?,type,name,?,file_path,start_line,end_line,start_byte,end_byte FROM nodes',('alias',''))
        node,_=c.select_seed(self.db,self.source,self.files,'canonical')
        self.assertEqual(node['node_id'],'id')
        with self.assertRaises(ValueError):c.select_seed(self.db,self.source,self.files,'wrong-canonical')


class OwnedReaderRestore(unittest.TestCase):
    def test_parent_admission_restored_without_candidate_payload_change(self):
        with tempfile.TemporaryDirectory() as tmp:
            parent=Path(tmp);version=parent/'data';version.mkdir()
            payload=version/'reader-protocol.json';payload.write_text('{"protocol":"reader-flock-v1"}')
            before=payload.read_bytes();c.restore_reader_coordination(version)
            self.assertEqual(payload.read_bytes(),before)
            self.assertEqual(sorted(p.name for p in version.iterdir()),['reader-protocol.json'])
            for p in (parent/'.reindex.lock',parent/'.readers/data.lock'):
                self.assertTrue(p.is_file());self.assertEqual(p.stat().st_nlink,1)
                self.assertEqual(p.stat().st_mode & 0o777,0o600)


if __name__=='__main__':unittest.main()
