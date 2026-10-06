# 승인된 B0 개발 기준선과 B1/C0 재검증 결과

2026-10-06 · **DEV 실제 수집·독립 자동 검사 완료 / FINAL 결과는 별도 보고 / HC 해석 승인·시험 preview 종료 결정 완료**. 성공한 도구 호출을 정답 성공으로 바꾸지 않는다. FINAL 결과를 이 개발 보고서에 섞지 않는다.

승인 입력은 [입력 승인 기록](./B0-APPROVED-INPUTS.md), 개발 동결은 [동결 원장](../../system/eval/b0-knowledge-system/approved-evaluation-m2max-2026-10-06/fix28-report-binding/development-freeze.json), 전체 원자료·SHA는 [개발 manifest](../../system/eval/b0-knowledge-system/approved-evaluation-m2max-2026-10-06/development-after-fix26-27/artifact-manifest.json)와 [native 오라클 manifest](../../system/eval/b0-knowledge-system/approved-evaluation-m2max-2026-10-06/development-oracles/artifact-manifest.json)에 있다. 큰 원자료는 lossless gzip이며 압축 전후 SHA가 별도로 남는다.

## 입력·환경·분모

- 고정 소스 71cb71cd55960833e930269e272f7a4a060be3aa/tree f020f8f30fd209b8de045756dedd12ff83f65cd9. 승인 Go29를 포함한 정적1,598파일/13,820벡터, DEV8 및 FINAL8 각150벡터 strict 빌드/원문·분할·orphan 감사 완료.
- BGE-M3 digest7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab,1024차원, Ollama0.35.1,registry,ctx/batch8192. M2 Max/macOS26.6.2,12 logical CPU/68GB; 공유 호스트이며 독점 성능 환경으로 주장하지 않는다.
- 실행 CKS SHA3b80c36d77b5c052894058ec59b12d64f2e5f9e5800aa11314be9e362189d0e6. 입력5개·벡터/그래프/의미 저장소/보관 메타데이터와 바이너리의 전후 SHA 불변.
- 8arms를 매 반복 회전. 각 요청 warmup2/검색5/warm20/cold3. 동적12요청·8상태2,880행 +정적4요청960행 =3,840행. 모델은 유지하며 cold는 새 CKS 프로세스까지의 시간이다.
- 여섯 동적 가족을 여섯 독립 unit으로 묶는다. F01 필터/exact@5와 F03 proposed 비승격은 양성 composed citation gold가 아니므로 그 인용 점수 분모에서 제외한다. 인용 품질 독립 unit은 정적4+동적4=8개다. 반복·언어·상태 변형을 독립 표본으로 부풀리지 않는다.
- 정적 코퍼스에는 승인 의미 저장소/팩이 없다. on 폴백의 동일 인용을 의미 팩의 개선으로 집계하지 않는다. 코퍼스 이름 knowledge-system과 평가 프로젝트 ID b0-knowledge-system은 독립 strict 감사의 전체 dataset identity에 결합한다.

## 정적 기준선: 실패를 보존

| 문항 | v1 검색 | v2 Recall@10 | v2 기대 인용 평균 역순위 | raw CKV 전체 exact 순위 |
|---|---|---:|---:|---|
| B0-CODE-01 | miss | 0.00 | 0.00 | 1085 |
| B0-WHY-03 | pass | 1.00 | 0.50 | 1 |
| B0-POLICY-01 | miss | 0.00 | 0.00 | 47/52 |
| B0-TRACE-01 | miss | 0.00 | 0.00 | 301/1490 |

네 DEV 문항의 평균 Recall은0.25, 평균 기대 인용 역순위는0.125다. 세 miss의 기대 source span은 이미 primary raw CKV top10 밖이며, 출처 누락/offset/필터/DB orphan 오류로 확인되지 않았다. 모델·키워드/RRF 후보 검색의 현재 제한으로 기록한다. 질문·gold·K·임계치를 성공에 맞추지 않았다. WHY raw1위가 composed2위이며 MRR 정의는 첫 관련 항목 하나가 아닌 기대 인용별 역순위 평균이다. 모든 새8arms의 정적 인용 지표가 같다.

## 동적·안전 자동 검사

- 2,880개 dynamic 및960개 static 공개 Go EvidencePackV2.Verify 오류0; source/citation/body SHA·좌표 감사 오류0; backend measurement ID 누락0.
- F02 모든480응답에 승인 꼬리 body 보존. raw exact와 KNN 순서 차이2건은 동일 거리 noise 청크 사이뿐이다. 같은 topK 집합·거리와 tail1위를 확인했으며 원래 exact_agreement=false도 유지했다.
- F03 모든720응답에서 proposed core의 규범적 사용/ontology boost0, baseline source citation/body 보존. 이는 검토된 운영 의미나 새로운 단일 개념 확정의 승인으로 바뀌지 않는다.
- F04 old/new480응답에서 자기 상태 marker만 사용. 과거 CKV를 새 소스와 직접 조회한 실제 오라클에서 old commit·fresh=false·stale_citation=true를 확인했다.
- F05 A/B480응답의 자기 프로젝트 body/정책, F06480응답의 code/policy·미검토 acceptance/test trace 비승격을 확인했다. 정책 on240회씩 실제 applicable policy가 보인다.
- 현재 SDK 상태 제어10호출: 정상6/부정4. 상태·프로젝트 혼입은 snapshot_mismatch, 손상 source는 snapshot_mismatch, missing source는 source_missing. 부정 경로는 backend 시도0/합성 private payload 비노출; 원본·다른 dataset과 복원 copy payload SHA 유지.
- raw CKV16요청의 전체 eligible inventory/1024 query vector/exact/상한·전후 DB/model/manifest SHA를 보관. F01 실제 sparse 자격·filtered exact@5·candidate limit은 별도 native 증거이며 public v2에는 filter 인자가 없다.
- 새 v1 anchor eval4×5와 별도 v1 SDK20호출의 measurement ID/backend/source 결합을 보관. v1 인용0 guard와 사람 답변 기권은 다른 판정이다. fixture marker 검사와 기존 sanitization/권한 회귀 범위를 일반 비밀 탐지·전체 운영 ACL 보증으로 확장하지 않는다.

## 지연과 비용

아래 각 population의 모든 성공 warm 관측을 nearest-rank p95로 계산했다. percentile·비율을 평균하지 않았다. static은 요청4×20=80, dynamic은12×20=240, pooled는320관측/arm이다. 검색/warmup/cold는 warm에 섞지 않는다. 호출 수·response 크기·cold 원값은 [합산 보고](../../system/eval/b0-knowledge-system/approved-evaluation-m2max-2026-10-06/development-after-fix26-27/development-summary.json)에 있다.

| arm | static p95 초 | static 비율 | dynamic 비율 | pooled 비율 |
|---|---:|---:|---:|---:|
| baseline_off | 4.291 | 1.000 | 1.000 | 1.000 |
| baseline_on | 4.850 | 1.130 | 1.315 | 1.078 |
| concept_text_off | 4.448 | 1.037 | 1.015 | 1.005 |
| concept_text_on | 4.706 | 1.097 | 1.549 | 1.074 |
| relations_off | 4.532 | 1.056 | 1.002 | 0.999 |
| relations_on | 5.200 | 1.212 | 1.435 | 1.100 |
| combined_off | 4.467 | 1.041 | 1.052 | 1.012 |
| combined_on | 5.545 | 1.292 | 1.614 | 1.065 |

Static combined_on1.292와 dynamic knowledge-on1.315–1.614는1.25 경계를 넘는다. pooled 값이 경계 안이어도 소형·정적 모집단의 비용 문제를 지우지 않는다. 입력 프로토콜에 없던 사후 가중치/모집단 선택으로 unconditional pass를 선언하지 않는다. 작은 중요 질문군은 승인 규칙대로 inconclusive이며 ontology 기본값 disabled 유지·출시 합격 없음이다. 사람 검토 비용은 미측정/null이며0초나0비용으로 대체하지 않는다.

## C0 관측 실패·수정과 동결

FIX22 승인 Go29 opt-in 발견, FIX23 bounded stderr/stdio, FIX24 default-provider preflight, FIX25 v2 selected-body budget, FIX26 같은 canonical peer 대비 중복 header boost, FIX27 요청 내 retained batch를 수정·검증했다. FIX26의 기대 인용 MRR .6666667→.75가 모든8arm에서 회복됐다. FIX27 실제 정적 SDK4개는 팩 전체 바이트 동일, 호출10.24/9.99/7.66/10.36초→2.99/4.55/2.09/3.90초였다. 이4개 단발 비교는 formal p95가 아니다.

수정 전 실패·관련 시험/race/vet·root Go 통과와 역사적 불완전 frozen-source 모듈 경계/원문5파일 보존을 [FIX26/27](./C0-ONTOLOGY-HEADER-AND-ARCHIVE-BATCH.md)에 연결했다. Python B075 및 B1보고26시험 통과. FIX28은 평가 보고만 수정: 전체 strict identity audit를 요구하는 별도 project ID, public v2 tool의 명시/생략 표현을 구분하고 잘못된 audit·외국 identity·쓰기 tool은 거부한다. real 승인 파일을 가정하던 pending 시험은 사본에 pending을 명시한다. 첫 보고/시험 실패도 [FIX28 원장](../../system/eval/b0-knowledge-system/approved-evaluation-m2max-2026-10-06/fix28-report-binding/artifact-manifest.json)에 보존했다.

개발에서 확인한 실제 계약/순위/반복 원문 비용 결함의 수정과 자동 검증을 동결한다. baseline recall 제한, optional latency/소표본 제한은 해결된 품질 성공으로 바꾸지 않는다. 최종 입력은 별도8정적+6동적 가족/8상태이며4,800회를 같은 바이너리·모델·K·회전·반복으로 실행한다. FINAL을 보고 소스·query·gold·threshold를 튜닝하지 않는다.

## 사람 판정과 종료 범위

[HC01–11](./B1-APPROVED-OUTPUT-CLAIM-REVIEW.md)의 실제 해석/사람 기권을 사용자가 승인했다. [paired 비교 종료](./B1-HUMAN-AND-PAIRED-CLOSURE.md)에 B1-04/07/08의 분모·실패·불확실·비용과 연결했다. 사람 비용은 미측정/null이며 생성 답변 오류율을 전체 응답0으로 만들지 않았다.

사용자가 시험 preview로 종료를 선택했고 전체30/30·승인 범위 잔여0이다. [최종 지원·종료 보고](./C1-FINAL-DELIVERY-REVIEW.md)에 운영 제외/미승인 항목을 기록했다. B0 기준선과 C0 수정/회귀/동결은 pass 범위, B1/C1 품질 fail·중요 그룹 inconclusive·disabled·운영 출시보류는 그대로다.
