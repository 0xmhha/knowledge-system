#!/usr/bin/env python3
"""Preregister source-function retrieval criteria; not human/independent quality gold."""
import argparse
import hashlib
import json
from pathlib import Path
import sqlite3
import subprocess
ROOT=Path(__file__).resolve().parents[1]
VERSION=Path('/private/tmp/ks-ontology-effect-20261015/version')
REPO=Path('/private/var/folders/rm/hlzw3w710ml8tk5t0s79rhtc0000gn/T/ks-o03-accepted-i10n284z/source')
UNITS={
 'quorum-f':('consensus/wbft/validator/default.go','validator.defaultSet.F'),
 'quorum-size':('consensus/wbft/validator/default.go','validator.defaultSet.QuorumSize'),
 'round-broadcast':('consensus/wbft/core/roundchange.go','core.Core.broadcastRoundChange'),
 'round-collect':('consensus/wbft/core/roundchange.go','core.roundChangeSet.Add'),
 'handler-preprepare':('consensus/wbft/core/preprepare.go','core.Core.handlePreprepareMsg'),
 'handler-prepare':('consensus/wbft/core/prepare.go','core.Core.handlePrepareMsg'),
 'handler-commit':('consensus/wbft/core/commit.go','core.Core.handleCommitMsg'),
 'current-read':('consensus/wbft/core/core.go','core.Core.IsCurrentProposal'),
 'current-round':('consensus/wbft/core/core.go','core.Core.startNewRound'),
 'validator-size':('consensus/wbft/validator/default.go','validator.defaultSet.Size'),
 'validator-list':('consensus/wbft/validator/default.go','validator.defaultSet.List'),
}
FAMILIES={
 'quorum':(['quorum-f','quorum-size'],['wbft-quorum']),
 'round-change':(['round-broadcast','round-collect'],['wbft-round-change']),
 'message-handlers':(['handler-preprepare','handler-prepare','handler-commit'],['wbft-message-handlers']),
 'current-lock':(['current-read','current-round'],['wbft-current-lock']),
 'validator-lock':(['validator-size','validator-list'],[]),
}
QUESTIONS=[
 ('EX-01','quorum','ko_registered','쿼럼 크기를 정하는 실제 F와 QuorumSize 계산 함수 두 곳을 찾아라.'),
 ('EX-02','quorum','en_registered','For the consensus threshold, locate both functions that calculate F and the quorum size in this pinned WBFT source.'),
 ('EX-03','quorum','ko_paraphrase','검증 참여자 수에서 허용 실패 수를 빼고 올림하는 최소 투표 기준의 계산 함수와 실패 수 계산 함수를 찾아라.'),
 ('EX-04','round-change','ko_registered','라운드 체인지에서 이전 prepared 증거를 송신하는 함수와 가장 높은 준비 라운드/블록을 수신 집합에 보관하는 함수를 찾아라.'),
 ('EX-05','round-change','en_registered','For view change, locate the function broadcasting prior prepared evidence and the collector retaining the highest prepared round and block.'),
 ('EX-06','round-change','ko_paraphrase','다음 차수로 넘어갈 때 이전 인증 자료를 내보내는 곳과 수신 자료 중 가장 큰 준비 번호와 블록을 기억하는 함수를 찾아라.'),
 ('EX-07','message-handlers','ko_registered','프리프리페어·프리페어·커밋 메시지에서 각각 상태 전이를 수행하는 세 처리 함수를 모두 찾아라.'),
 ('EX-08','message-handlers','en_registered','Locate all three BFT message handlers that apply preprepare, prepare and commit phase state transitions.'),
 ('EX-09','message-handlers','ko_paraphrase','제안 수신 처리, 첫 투표 누적, 마지막 서명 집계 단계에서 상태가 전이되는 처리 함수 세 곳을 찾아라.'),
 ('EX-10','current-lock','ko_registered','락 규율을 확인하려고 한다. 현재 제안 해시를 검사하는 함수와 새 라운드를 시작하며 현재 상태 포인터를 보호하는 함수를 찾아라.'),
 ('EX-11','current-lock','en_registered','For lock discipline, locate the current proposal hash accessor and the new round transition function that holds currentMutex.'),
 ('EX-12','current-lock','ko_paraphrase','바뀔 수 있는 현재 상태 객체의 포인터를 읽는 곳과 새 차수로 교체할 때 포인터를 보호하는 함수 두 곳을 찾아라.'),
 ('EX-13','quorum','comment_implementation','쿼럼 주석의 ceil(2N/3) 표기와 구현을 구분하려고 한다. 실제 F와 QuorumSize 반환식을 확인할 수 있는 두 계산 함수를 찾아라.'),
 ('EX-14','validator-lock','ambiguous_other_entity','WBFT current 포인터가 아니라 validatorMu의 RWMutex를 확인한다. 검증인 집합의 크기와 목록을 읽으며 validatorMu.RLock을 잡는 Size와 List 함수를 찾아라.'),
 ('EX-15',None,'unknown_definition','이 평가에 등록된 검토된 온톨로지 정의나 구현 연결에 staking 해제 대기 기간의 근거가 있는가? 등록되지 않은 내용을 기능 부재라고 단정하지 말고 범위 밖을 구분하라.'),
 ('EX-16',None,'foreign_scope_term','질의 대상은 go-wemix의 voting quorum 정책이며 WBFT는 대상에서 제외한다. 이 고정 WBFT 코퍼스만으로 다른 프로젝트의 정책을 답하지 말고 등록 범위 밖을 구분하라.'),
]


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--out',type=Path,required=True);out=parser.parse_args().out.resolve()
    if out.exists():raise ValueError('fresh output required')
    out.mkdir()
    def sha(data):return hashlib.sha256(data).hexdigest()
    def write(name,v):(out/name).write_text(json.dumps(v,ensure_ascii=False,indent=2)+'\n')
    identity=json.loads((VERSION/'dataset-identity.json').read_text());manifest=json.loads((VERSION/'sources/manifest.json').read_text());files={(r['origin_id'],r['path']):r for r in manifest['files']};project=identity['source']['project_id'];commit=identity['source']['source_commit']
    if subprocess.check_output(['git','-C',str(REPO),'status','--porcelain=v1']).strip():raise ValueError('source repo not clean')
    if commit!='baca2869118774c5f6219932dc76e197a47e6ecd' or subprocess.check_output(['git','-C',str(REPO),'rev-parse','HEAD']).decode().strip()!=commit:raise ValueError('source commit differs')
    actual=json.loads((ROOT/'system/eval/knowledge-roadmap-2026-10-16/km-06-real-projects/runtime/fixtures.json').read_text())[0];other=json.loads((Path(actual['version'])/'sources/manifest.json').read_text())
    def gos(m):return{r['path']:r['sha256']for r in m['files']if r['origin_id']=='repo'and r['path'].endswith('.go')and not r['path'].endswith('_test.go')}
    if gos(manifest)!=gos(other)or len(gos(manifest))!=882:raise ValueError('non-test Go corpus differs')
    ontology=ROOT/'projects/stablenet/domain-knowledge/meaning-approved/ontology.yaml';retained=files[('repo','docs/approved-meaning/ontology.yaml')]
    if retained['sha256']!=sha(ontology.read_bytes())or(REPO/'docs/approved-meaning/ontology.yaml').read_bytes()!=ontology.read_bytes():raise ValueError('approved meanings changed')
    units=[]
    with sqlite3.connect('file:'+str(VERSION/'graph/graph.db')+'?mode=ro',uri=True)as db:
        for key,(file,name)in UNITS.items():
            rows=db.execute("SELECT name,start_line,end_line,canonical_id FROM nodes WHERE file_path=? AND qualified_name=? AND type IN('Method','Function')",(file,name)).fetchall()
            if len(rows)!=1:raise ValueError('unit not unique '+key)
            local,start,end,canonical=rows[0];rec=files[('repo',file)];source=(VERSION/'sources/blobs'/rec['sha256']).read_bytes();span=b''.join(source.splitlines(keepends=True)[start-1:end])
            if sha(source)!=rec['sha256']or local not in span.decode()or 'func 'not in span.decode():raise ValueError('unit source span differs')
            units.append({'id':key,'file':file,'start_line':start,'end_line':end,'canonical_id':canonical,'qualified_name':name,'file_sha256':rec['sha256'],'span_sha256':sha(span),'expected_code_body':span.decode()})
    oracle={'schema_version':1,'kind':'preregistered_finite_function_retrieval_criteria_not_general_semantic_gold','author':'assistant_source_derived','human_approval':False,'independent_quality_gold':False,'independent_units_approved':0,'units':units,'families':{k:{'unit_ids':u,'intended_ontology_concepts':c}for k,(u,c)in FAMILIES.items()},'scoring':{'unit_hit':'a top10 unique published source citation covers the complete expected function span and contains its verified function body','precision':'unique published citation item with >=1 complete expected function and its body / all unique published citation items in top10; no items=0; macro question/family averages','recall':'unique expected function units fully covered / declared expected units for this code-location question','rank':'first relevant returned evidence reciprocal rank and hit@10','not_scored':'EX15/16 unknown/foreign questions have no exhaustive source relevance set; only ontology-intent scope and source integrity are observed','limits':'closed-world function-finding tasks; not exhaustive behavior/policy relevance or generated answer quality'}};write('private-function-criteria.json',oracle)
    requests=[];labels=[]
    for qid,family,kind,prompt in QUESTIONS:
        units_expected,concepts=FAMILIES[family]if family else([],[]);requests.append({'id':qid,'tool':'cks.context.get_for_task_v2','arguments':{'prompt':prompt,'explain_ontology':True}});labels.append({'id':qid,'family':family,'kind':kind,'expected_unit_ids':units_expected,'intended_ontology_concepts':concepts,'retrieval_scored':family is not None})
    write('requests.json',{'schema_version':1,'kind':'expanded_development_requests_not_independent_holdout','requests':requests});write('labels.json',labels)
    preserved={str(VERSION/name):sha((VERSION/name).read_bytes())for name in('graph/graph.db','vector/vector.db','graph/manifest.json','vector/manifest.json','dataset-identity.json','sources/manifest.json','reader-protocol.json','review-hold.json')}
    source_pins={'version_dir':str(VERSION),'source_repo':str(REPO),'source_commit':commit,'project_id':project,'dataset_id':identity['dataset_id'],'source':identity['source'],'approved_ontology_sha256':sha(ontology.read_bytes()),'non_test_Go_equal_to_0bf2':882,'preserved_files':preserved,'binary_sha256':sha((ROOT/'bin/cks').read_bytes())};write('source-pins.json',source_pins)
    protocol={'schema_version':1,'stage':'preregistered_no_new_query_execution','question_count':16,'retrieval_scored_questions':14,'unscored_source_controls':2,'fact_families':5,'unique_function_units':11,'approved_concepts':4,'approved_implementation_links':0,'arms':['off','concept_text','relations','combined'],'counts':{'warmup':1,'retrieval':2,'warm_latency':5,'cold_process':0},'planned_capture_calls':512,'API_instrumentation':{'mode':'owned loopback HTTP proxy to approved local Ollama127.0.0.1:11434','attribution':'question-arm batch; metadata/readiness separated','recorded':'path/status/request SHA256 and byte length, elapsed time, optional server-reported prompt_eval_count','token_counts_inferred_from_bytes':False,'latency_scope':'same proxy for all arms; local proxy overhead included; descriptive warm latency'},'mode_order':'rotate per question index','scoring_cap':10,'search_caps':'identical non-mode generated configuration; actual settings recorded before capture','model':{'name':'bge-m3:latest','digest':'7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab','dimension':1024},'requests_sha256':sha((out/'requests.json').read_bytes()),'labels_sha256':sha((out/'labels.json').read_bytes()),'criteria_sha256':sha((out/'private-function-criteria.json').read_bytes()),'quality_acceptance':False,'independent_gold_or_holdout':False,'LLM_answer_quality':None,'paid_model_calls':0,'default_ontology_mode_changed':False,'limitations':['questions authored after prior no_match results; mechanism-focused development study, not untouched holdout','same fact language variants and repeated calls are clustered, not independent new facts','source-derived finite function-finding oracle, not human accepted general behavior/policy relevance','old13/42 and old12 questions are not reused as independent new question data; reference families overlap and are disclosed','relations arm has zero reviewed implementation assertions; execution does not create missing semantic links','local shared host small warm sample; descriptive latency only; independent acceptance remains separate']};write('protocol.json',protocol)
    review=['# 온톨로지 확대 평가 사전 등록','','현재는 질문/범위/함수 기준의 준비이며 새 검색 결과·품질 승인·사람 정답이 아니다. 함수 찾기14질문과 source relevance 미채점2제어, 사실군5개·고유 함수11개다. 표현 변형과 반복을 독립 사실로 세지 않는다.','','| ID | 종류 | 질문 | 함수 기준 |','|---|---|---|---|']
    for request,label in zip(requests,labels,strict=True):review.append('| '+request['id']+' | '+label['kind']+' | '+request['arguments']['prompt']+' | '+(', '.join(label['expected_unit_ids'])or'검색 relevance 미채점; 범위 관찰')+' |')
    review.extend(['','기준은 검색 전에 고정하며 source-only 함수 회수 precision/recall·순위와 ontology 의도 범위·warm 지연을 기록한다. 일반 답변의 의미 정확도·운영 품질·독립 정답으로 확대하지 않는다.4모드는 동일 corpus/source/model/config를 사용하고 차이는 ontology mode뿐이다. 총512회는16개 질문의 반복이며 독립512개 정답이 아니다.']);(out/'review.md').write_text('\n'.join(review)+'\n')
    print('prepared16 questions14 scored2 controls5 families11 source functions; new retrieval calls0')

if __name__=='__main__':main()
