from pathlib import Path
import json,gzip,shutil,hashlib,subprocess,importlib.util
from datetime import datetime,timezone
root=Path.cwd();run=Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/comparison-go-stablenet-20261013');packet=root/'system/eval/stablenet-dataset-comparison-2026-10-13';p=json.load(open(run/'protocol.json'));summary=json.load(open(run/'capture/summary.json'));extra=json.load(open(run/'extra-summary.json'));sha=lambda path:hashlib.file_digest(Path(path).open('rb'),'sha256').hexdigest()
spec=importlib.util.spec_from_file_location('comparator',root/'projects/stablenet/eval/dataset-comparison/run.py');M=importlib.util.module_from_spec(spec);spec.loader.exec_module(M);M.baselines_guard(p);M.model_guard(p)
assert subprocess.check_output(['git','-C',p['source'],'rev-parse','HEAD'],text=True).strip()==p['source_commit']
assert summary['responses']==1512 and summary['both_valid_pairs']==396
assert all(a['observed']==756 and a['missing']==0 and len(a['invalid_packs'])==0 for a in summary['arms'].values())
legacy=json.load(open(run/'legacy-diagnostic-response.json'));pack=legacy['rows'][0]['call']['response']['structuredContent'];version=Path(p['baselines'][0]['version']);known={f['path']for f in json.load(open(version/'sources/manifest.json'))['files']if f['origin_id']=='repo'}
missing=[]
for b in pack['bodies']:
 c=b['citation']
 if c['file']not in known:missing.append({'file':c['file'],'start_line':c['start_line'],'end_line':c['end_line'],'in_retained_repo_manifest':False})
ref=json.load(open(run/'diagnostic/refinputs-v2-response.json'));response=ref['rows'][0]['call']['response'];assert not response.get('isError');M.verify_pack(response['structuredContent'],Path(p['baselines'][1]['version']))
for block in [1,2]:
 for arm in ['baseline','candidate']:
  source=run/'capture'/str(block)/arm;target=packet/'capture'/str(block)/arm;target.mkdir(parents=True,exist_ok=True)
  for filename in ['config.yaml','command.json','capture.json.gz']:shutil.copy2(source/filename,target/filename)
  events=source/'events/cks-mcp.jsonl'
  with events.open('rb') as src,gzip.open(target/'backend-events.jsonl.gz','wb')as dest:shutil.copyfileobj(src,dest)
  shutil.copy2(run/'requests.json',target/'requests.json')
(packet/'raw-paired-summary.json').write_bytes((run/'capture/summary.json').read_bytes());(packet/'matched-request-summary.json').write_bytes((run/'extra-summary.json').read_bytes())
for label,path in [('baseline-legacy',run/'legacy-diagnostic-response.json'),('refinputs-legacy',run/'diagnostic/refinputs-legacy-response.json'),('refinputs-v2',run/'diagnostic/refinputs-v2-response.json')]:
 with path.open('rb')as src,gzip.open(packet/(label+'-diagnostic.json.gz'),'wb')as dest:shutil.copyfileobj(src,dest)
verification=json.load(open(packet/'candidate-verification.json'));old=summary['arms']['baseline'];new=summary['arms']['candidate'];ratio=new['warm_p95_ms']/old['warm_p95_ms']
record={'finished_at':datetime.now(timezone.utc).isoformat(),'scope':'real same-source dataset rebuild and engineering comparison; errors retained, no independent generated-answer quality verdict','source_commit':p['source_commit'],'candidate_dataset':p['candidate_dataset'],'candidate_version':p['candidate_version'],'model':p['model'],'responses':1512,'baseline':{'observed':756,'failed':old['failed'],'failure_rate':old['failed']/756,'failure_codes':extra['arms']['baseline']['failure_codes'],'valid_packs':old['valid_packs'],'successful_warm_n':old['warm_n'],'successful_warm_p95_ms':old['warm_p95_ms'],'expected_signature_hit_at_10':old['source_derived_anchor_hit_at_10'],'expected_signature_RR_at_10':old['source_derived_anchor_RR_at_10'],'request_HTTP_attempts':extra['arms']['baseline']['request_HTTP_attempts']},'candidate':{'observed':756,'failed':new['failed'],'failure_rate':new['failed']/756,'failure_codes':extra['arms']['candidate']['failure_codes'],'valid_packs':new['valid_packs'],'successful_warm_n':new['warm_n'],'successful_warm_p95_ms':new['warm_p95_ms'],'expected_signature_hit_at_10':new['source_derived_anchor_hit_at_10'],'expected_signature_RR_at_10':new['source_derived_anchor_RR_at_10'],'request_HTTP_attempts':extra['arms']['candidate']['request_HTTP_attempts']},'both_valid_pairs':396,'citation_order_changed_pairs':summary['citation_order_changed_pairs'],'body_outputs_changed_pairs':summary['body_outputs_changed_pairs'],'successful_warm_p95_ratio':ratio,'per_question_successful_p95_ratio_range':[min(extra['successful_warm_p95_ratio_by_question'].values()),max(extra['successful_warm_p95_ratio_by_question'].values())],'structural_comparison_all_rows_equal':True,'keyword_comparison':json.load(open(packet/'keyword-summary.json')),'candidate_verification':verification,'source_missing_representative_diagnostic':{'legacy_body_refs_absent_from_retained_repo':missing,'same_query_refinputs_v2_success':True,'refinputs_v2_citations':len(response['structuredContent']['citations']),'finding':'generated domain-corpus entries used by legacy composer are outside retained repo source lookup; v2 build refuses these references and returns source_missing','scope_limit':'representative prepare-handler probe; not every failed prompt traced independently'},'all_original_baseline_DB_SHA_current_and_source_HEAD_preserved':True,'new_existing_operator_dataset_activation':False,'run_notes':['new current pointer belongs only to the separate comparison dataset root','diagnostic used the first baseline logging directory after scored captures; final request-cost counters match exact scored measurement IDs and exclude diagnostics/startup','canonical ratio directly verified; disabled threshold gate not relabelled as passed','30 reused prompts and12 mechanical signature anchors; no generated answer scoring or independent quality approval','timing successful responses only:220/420 warm samples per arm;420 total include failures','two alternating arm blocks, same warm model/shared host,10 timing samples per question; performance observations not acceptance'], 'next_work':'retain both datasets; first fix generated-domain citation provenance in v2 serving, then repeat the same42 question comparison'}
(packet/'result.json').write_text(json.dumps(record,ensure_ascii=False,indent=2)+'\n')
doc=root/'docs/spec-driven/GO-STABLENET-DATASET-COMPARISON.md';text=doc.read_text();start=text.index('## 작업표와 완료 기준');text=text[:start]+'''## 작업표와 실행 증거

| ID | 상태 | 확인한 실행 증거 |
|---|---|---|
| DC-01 | 완료 | 소스·기존3개 버전/DB SHA·모델·바이너리·팩·42질문/8fixture 고정 |
| DC-02 | 완료 · gate 경고 기록 | 실제26분16초 build, graph/vector 좌표·strict embeddings·DB hash·canonical 직접 비율 확인 |
| DC-03 | 완료 | strict doctor ready, health28도구·실제 Info 질의 성공; 기존 DB/current/HEAD 보존 |
| DC-04 | 완료 | 기존 주 기준과 구조 비교 전 항목 동일; 참고 입력 차이 분리 |
| DC-05 | 완료 · fixture1개 실패 보존 | 네 graph 모두8개 중7개, recall1.0/precision0.735294/F1 0.847458 |
| DC-06 | 완료 · API 오류 보존 | 1512응답·누락0; 각756회 중360 source_missing; 정상792개 pack 검증,396쌍 비교 |
| DC-07 | 완료 | 결과·대표 오류 경로·한계·다음 작업 및 원자료/SHA를 기록. 커밋 조회 근거는 진행 원장을 따름 |

완료는 데이터 생성과 비교 실행·오류 판정을 기록했다는 뜻이다. 전체 질문의 정상 동작이나 품질 개선을 수용했다는 뜻은 아니다.

## 비교 결과

| 지표 | 기존 주 기준 | 새 데이터 |
|---|---:|---:|
| graph 파일 / 노드 / 관계 | 1323 / 244822 / 1963773 | 동일 |
| vector 청크 / domain / flow | 21304 / 346 / 96 | 동일 |
| symbol canonical | 18691/18837 (99.2249%) | 동일 |
| keyword8개 통과 | 7/8 | 7/8 |
| keyword recall / precision / F1 | 1.0 / 0.735294 / 0.847458 | 동일 |
| 42질문 반복 호출 | 756 | 756 |
| source_missing 오류 | 360 (47.619%) | 360 (47.619%) |
| 정상 v2 pack·원문/좌표/hash 확인 | 396 | 396 |
| source-derived12개 signature hit@10 / RR@10 | 0.5 / 0.388889 | 동일 |
| 성공 응답 warm p95(ms), n220 | 1238.934 | 1191.894 |
| 요청 scope embed / tags 시도 | 3138 / 6276 | 3136 / 6272 |

정상396쌍의 인용 순서·본문 변화0. 구조·정상 검색 결과·오류율에서 개선 차이를 관측하지 못했다. 전체 성공 p95 비율은 약0.9620이지만 질문별 비율 범위는0.7664–1.9370이며 성공한22질문만 지연 집계에 들어간다. 반복10회·warm/shared-host·교대 순서의 기술 관측이다. 성능 향상 수용으로 바꾸지 않는다.

SNK06 mint allowance는 기대 systemcontracts6개를 모두 회수했지만 test.*9개가 추가되어 해당 precision0.4다. fixture는 전역 검색과 제한된 기대 목록을 사용한다. 원래 입력·실패를 그대로 유지하며 새 데이터의 회귀로 단정하지 않는다.

## source_missing 대표 원인과 다음 작업

prepare-handler 질문의 v1 본문에는 `entries/A1.wbft_core.message_handlers.md`가 포함된다. 이 파일은 generated domain corpus에 있으며 retained repo manifest에는 없다. v2 생산자는 보관 원문의 repo 경로로 이 참조를 읽으려다 source_missing을 반환한다. 동일 질문은 domain/flow가 없는 refinputs 데이터의 v2에서 정상11인용을 반환했다. 대표사례의 실제 경로 진단이며20개 실패 질문의 모든 경로를 각각 조사했다는 뜻은 아니다.

우선순위1: generated domain/flow 인용의 origin·캡처·namespace·v2 본문 매핑 계약을 보완하고, 오류가 난42질문을 동일 조건으로 다시 실행한다. 기존 데이터와 원자료를 보존한 상태에서 수정·회귀 기준을 고정한다. 이후 필요하면 SNK06 기대 범위와 실제 도구의 검색 범위를 별도 검토한다.

사용자가 별도로 보류한 출시 사항은 요청 전까지 일반 진행 보고에 포함하지 않는다. 이 작업의 진행/남은 일/다음 단계만 보고한다.

프로토콜과 실행 근거: [evaluation packet](../../system/eval/stablenet-dataset-comparison-2026-10-13/), [protocol](../../system/eval/stablenet-dataset-comparison-2026-10-13/protocol.json), [최종 결과](../../system/eval/stablenet-dataset-comparison-2026-10-13/result.json). 실제 경로와 검증 바이너리는 프로토콜을 따른다.
''';doc.write_text(text)
print('final result written; baselines and source preserved; complete7 comparison tasks; outstanding source-provenance correctness issue')
