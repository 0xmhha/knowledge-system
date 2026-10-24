# 현재 인수인계

기준: 2026-10-23, main `0a70f502`(PR #124까지 머지). 이 문서는 지금 단계, 진행 중인 작업, 바로 다음 행동만 적는다. 남은 전체 목록, 맥락, 하지 않을 일은 [남은 작업](../REMAINING-WORK.md)에 있고, 정본은 [실행 원장](./REMAINING-13-EXECUTION.json)이다. 새 머신이나 새 세션은 [머신 간 인수인계](./MACHINE-HANDOFF.md)로 환경부터 준비한다.

## 1. 흔들리면 안 되는 기준

이 시스템은 어떤 프로젝트든 지식 DB로 만들고, 느슨하게 묻는 질문(vibe query)에 찾는 코드를 거의 한 번에 압축해 돌려줘야 한다. 사용자가 2026-10-23에 우선순위를 정했다([결정 기록](./operational-preparation/retrieval-priority-2026-10-23/DECISION.json)).

1. 질문의 의도를 정확히 파악한다.
2. 그 의도의 코드와, 그 코드와 연결된 코드를 한 번에 찾아 준다.

테스트는 정답이 이미 확인된 사례로만 한다. 지금 하는 grpc-go 평가는 머지된 PR을 정답으로 쓴다. 지금 단계의 일은 이 두 가지를 숫자로 재는 것이다. 숫자가 나오기 전에는 검색 로직을 고치지 않는다. 결과를 본 뒤 표본, 정답, 질문을 바꾸지 않는다.

## 2. 현재 단계

원장 `current_activity`: `102_retrieval_eval_grpc_go_run1_results_recorded_sample_check_pending`. [남은 작업](../REMAINING-WORK.md) 3.1의 3단계(명세대로 실행)에서 실행은 끝났고 사람의 표본 확인을 기다린다. 작업은 로컬 브랜치 `test/retrieval-eval-grpc-go-run`에 커밋했고 push하지 않았다.

### 끝난 것

|PR|내용|
|---|---|
|#121|검색 평가를 1순위로 정한 결정과 WI-V1 정답 확인 기록|
|#122|`cks eval`에 `expected_intent`, `first_hit_rank`, `used_tokens`, `rank_summary`, `intent_accuracy`를 더함([계약](./EVAL-CONTRACT.md))|
|#123|grpc-go 입력 고정(PR 100건, 정답 범위 575개, 한국어 질문 100개, 스냅샷 24개)과 변환·비교 대상·연쇄 색인 스크립트|
|#124|복사한 벡터 색인이 이번 스냅샷의 그래프를 가리키게 고침. 모델 결정을 명세 3장에 기록|

[평가 명세](./RETRIEVAL-EVAL-SPEC.md)는 사용자가 승인했다. 입력은 [grpc-go 입력](./operational-preparation/retrieval-eval-grpc-go-v1/README.md)에 고정돼 있고 `manifest.json`이 해시를 묶는다.

### 임베딩 모델 (사용자 결정)

- 본 평가: `qwen3-embedding:4b`, 2560차원. 모델 기본 출력이 2560차원이라 `--embed-dim`을 주지 않는다.
- 기준선: `bge-m3`, 1024차원.
- 다른 모델은 받지 않는다. 사용자가 "그냥 다른 모델 받지 말고"라고 정했다. 제외한 모델과 이유는 [명세](./RETRIEVAL-EVAL-SPEC.md) 3장에 있다.
- Ollama에서 임베딩을 지원하는 모델은 이 둘뿐이다. `qwen3.5:9b`, `gemma4:12b` 등은 대화형 LLM이라 임베딩에 쓰지 않는다.

### 1차 실행 결과 (2026-10-24)

두 모델의 색인 48개와 CKS·비교 대상 실행을 마쳤다. 결과, 확인 방법, 실행 중 고친 것은 [1차 결과](./operational-preparation/retrieval-eval-grpc-go-v1/results/README.md)에 있다. 작업 폴더는 저장소 밖 `knowledge-data/grpc-go-retrieval-eval`이고, 앞선 실행의 출력(`eval-smoke`, `eval-without-responses`, `eval-interrupted-stale-ledger`)도 지우지 않고 거기 두었다.

- PR 100건 기준 hit@1: 벡터만 0.24(qwen3)·0.20(bge-m3), grep 0.14, CKS는 `cks eval` 그대로 0.07·0.03, 커밋 필터 없이 다시 채점하면 0.14·0.11
- CKS 의도 정확도: 0/100(qwen3), 14/100(bge-m3). 대부분 `unknown`
- 그래프 낱말 검색: 23건이 한국어 질문에서 낱말을 못 뽑아 오류

## 3. 색인을 만들고 이어 가는 법

### 준비

- 저장소 루트에서 `make build-bins`로 `bin/ckg`, `bin/ckv`, `bin/cks`를 만든다.
- grpc-go 전체 이력 clone을 저장소 밖에 둔다(`git clone https://github.com/grpc/grpc-go`).
- `ollama list`에 `qwen3-embedding:4b`와 `bge-m3`가 있어야 한다.
- 작업 디렉터리 `$WORK`는 저장소 밖에 둔다. 색인 하나가 수 GB라 커밋하지 않는다.

### 스냅샷 목록

`inputs/sample.json`의 `snapshots`는 해시 순이다. 연쇄 색인은 시간 순서가 필요하므로 커밋 시각으로 정렬해 `$WORK/snapshots.json`을 만든다.

```sh
D=docs/spec-driven/operational-preparation/retrieval-eval-grpc-go-v1
python3 -c "import json;print('\n'.join(json.load(open('$D/inputs/sample.json'))['snapshots']))" \
  | xargs git -C <grpc-go clone> show -s --format='%ct %H' | sort -n | cut -d' ' -f2 \
  | python3 -c "import json,sys;print(json.dumps(sys.stdin.read().split()))" > $WORK/snapshots.json
```

첫 항목은 `88ac7035`, 마지막은 `a9cbf17a`, 모두 24개다.

### 실행

```sh
python3 scripts/retrieval_snapshot_chain.py --repo <grpc-go clone> --snapshots $WORK/snapshots.json \
  --work $WORK --bin bin --model qwen3-embedding:4b
python3 scripts/retrieval_snapshot_chain.py --repo <grpc-go clone> --snapshots $WORK/snapshots.json \
  --work $WORK --bin bin --model bge-m3
```

두 모델을 동시에 돌리지 않는다. 같은 `src-chain` worktree를 checkout하기 때문이다. 그래프는 모델과 무관하다. 그래서 첫 모델이 만든 `chain-<스냅샷>/graph`를 둘째 모델이 다시 쓴다.

### 만들어지는 것

|경로|내용|
|---|---|
|`$WORK/src-chain`|grpc-go worktree. 스냅샷마다 checkout한다|
|`$WORK/chain-<스냅샷>/graph`|그래프 DB. 스냅샷마다 한 번 만든다|
|`$WORK/vec/<모델>/<스냅샷>`|모델별 벡터 색인. 모델 이름의 `:`와 `/`는 `-`로 바꾼다|
|`$WORK/chain-<모델>.json`|스냅샷마다 모드(full, reindex)와 걸린 시간|
|`$WORK/chain-<모델>-<스냅샷>.log`|빌드 로그|

### 걸리는 시간 (이 머신에서 잰 값)

- 그래프: 스냅샷 하나에 약 220초.
- `bge-m3` 첫 스냅샷 전체 빌드: 약 40분(임베딩 36분, 청크 15,860개).
- `qwen3-embedding:4b` 첫 스냅샷 전체 빌드: `bge-m3`보다 약 4배 느리다. 851/1082 파일에 2시간이 걸렸으므로 전체는 2시간 30분 안팎으로 본다. 끝나면 `chain-qwen3-embedding-4b.json`의 실제 값으로 이 줄을 바꾼다.
- `reindex`: 바뀐 파일 수에 따라 10초에서 2분 남짓(`bge-m3` 기준).

## 4. 바로 다음 행동 (이 순서를 따른다)

1. **사람의 표본 확인을 받는다.** 확인 자료는 [표본 확인](./operational-preparation/retrieval-eval-grpc-go-v1/sample-check/PACKET.md)에 있다(seed 20261024). 1차 결과는 이미 나왔으므로, 확인하는 사람에게 결과를 보기 전인지 함께 묻고 그대로 기록한다. 확인 결과는 사용자 메시지 그대로 기록하고, AI가 대신 확인하지 않는다. 확인 전의 결과는 DEV 근거다.
2. **스냅샷 커밋 판정을 어떻게 할지 사용자에게 묻는다.** `cks eval`은 스냅샷 커밋과 다른 커밋의 인용을 빼고, `reindex`한 색인은 바뀌지 않은 파일에 처음 임베딩한 커밋을 남긴다. 그래서 CKS 수치가 두 줄이다. 고르는 길은 색인을 스냅샷마다 전체로 다시 만드는 것(qwen3 기준 약 35시간)과, `cks eval`이 그 스냅샷에서 바뀌지 않은 파일의 인용을 받아들이게 고치는 것(평가 규칙 변경이므로 명세 갱신이 필요하다)이다.
3. 그다음 [남은 작업](../REMAINING-WORK.md) 3.1의 4단계(약한 곳 고치기)로 간다. 1차 결과에서 본 후보는 의도 분류 기준값(bge-m3에 맞춘 0.55), 그래프 낱말 검색의 한국어 처리, CKS가 벡터만 쓴 검색보다 낮은 원인이다. 고친 뒤 같은 표본으로 다시 잰다.
4. 5단계(go-stablenet)는 4단계 뒤에 한다.

단계를 하나 마칠 때마다 이 문서, [남은 작업](../REMAINING-WORK.md), 원장을 같은 커밋에서 갱신한다.

## 5. 이미 확인한 사실과 주의할 점

- **`reindex` 전에 그래프를 바꿔 가리켜야 한다.** 복사한 벡터 색인의 manifest는 앞 스냅샷의 그래프를 가리킨다. 그대로 `reindex`하면 같은 커밋을 전체 빌드한 것과 canonical ID가 629개 달랐다. 스크립트가 `sources.ckg.path`를 이번 스냅샷의 그래프로 바꾼 뒤 `reindex`한다. 그러면 code 청크는 전체 빌드와 같고, 패키지 convention 요약 청크 15,860개 중 12개만 다르다. 이 12개는 `ckv reindex`의 버그로 보고 나중에 따로 고친다. 평가 결과에는 이 차이를 함께 적는다.
- **`9c7231e8` 전의 `ckv`로 `reindex`한 색인은 그래프 커밋이 틀려 있다.** manifest의 `sources.ckg.src_commit`이 첫 스냅샷 커밋으로 남아 CKS가 모든 요청을 거절한다. 2026-10-24 색인 46개는 [1차 결과](./operational-preparation/retrieval-eval-grpc-go-v1/results/README.md)의 `index/fix-ckg-ledger.py`로 이 칸만 고쳤다. 새로 색인할 때는 고친 `ckv`를 쓴다.
- **8개 스냅샷의 그래프는 PR #124 전에 만든 것이다.** 그래프 빌드는 그 PR에서 바뀌지 않았다. 스크립트는 manifest에 그 커밋이 적혀 있으면 그래프를 다시 쓴다. 9번째 `d7b3f936`은 빌드 도중 멈춰 그래프가 없으므로 새로 만든다. 같은 작업 디렉터리의 옛 `chain.json`과 `chain-<스냅샷>/vector`는 그래프를 잘못 가리키던 `bge-m3` 색인이다. 평가에 쓰지 않는다.
- **긴 청크는 잘려서 임베딩된다.** Ollama가 긴 입력을 거부하면 `ckv`가 입력을 12,000바이트나 4,000바이트로 줄여 다시 보낸다. `qwen3-embedding:4b`는 851개 파일까지 16개 청크가 잘렸다. `bge-m3`는 같은 스냅샷에서 2개였다. 16개 중 13개가 큰 시험 파일, 2개가 생성 파일(`*.pb.go`), 1개가 예제 서버다. 시험 파일이 정답인 PR은 이 잘림의 영향을 받을 수 있다. 2026-10-24 실행의 색인 로그에는 잘린 청크와 건너뛴 청크가 두 모델 모두 0건이다.
- **`ckg query`는 한국어 질문에 자주 오류를 낸다.** 질문에서 쓸 수 있는 낱말을 못 찾으면 "no usable tokens"를 낸다. 명세대로 오류도 실패로 센다. 고치는 일은 4단계에서 한다.
- **부분 측정값은 참고만 한다.** `90f571db` 스냅샷의 18개 시나리오를 옛 `bge-m3` 색인으로 돌렸을 때 hit@1이 벡터 0.33, BM25 재정렬 0.17, `ckg query` 0, `git grep` 0이었다. CKS 시나리오 하나(pr-8792)는 정답 의도 refactor를 feature_add로 분류했고, 정답을 2위에 냈고, 3,030토큰을 썼다. 색인을 다시 만들기 전의 값이라 결과 보고에 쓰지 않는다.
- **`pplx-embed-v1-4b`는 쓰지 않는다.** 이 네트워크에서 Hugging Face에 닿지 않고, 양방향 구조라 Ollama에서 돌기 어렵다.

## 6. WI-V1 근거 (보류 중, 봉인된 패킷은 고치지 않는다)

|패킷|담은 것|
|---|---|
|[wiv1-reviewer-packet-v1](./operational-preparation/source-review-supplement/followups/wiv1-reviewer-packet-v1/REVIEW-PACKET.json)|검토자에게 넘긴 AI 초안과 빈 검토자 칸|
|[wiv1-contract-evidence-v1](./operational-preparation/source-review-supplement/followups/wiv1-contract-evidence-v1/REVIEW.json)|12계약마다 단계90~94 근거 인용 36개와 남은 공백|
|[wiv1-progress-true-generation-v1](./operational-preparation/source-review-supplement/followups/wiv1-progress-true-generation-v1/SUMMARY.json)|동기화 도중 true 알림의 세대 경계|
|[wiv1-notify-transport-generation-v1](./operational-preparation/source-review-supplement/followups/wiv1-notify-transport-generation-v1/SUMMARY.json)|Notify 도중 종료와 gate 없는 연결 종료|
|[wiv1-transport-timer-generation-v1](./operational-preparation/source-review-supplement/followups/wiv1-transport-timer-generation-v1/SUMMARY.json)|연결 종료와 같은 channel timer broadcast|
|[wiv1-generation-boundary-v1](./operational-preparation/source-review-supplement/followups/wiv1-generation-boundary-v1/SUMMARY.json)|owner ack 뒤 옛 세대 사용과 새 등록 경계|
|[factory-native-overlap-v1](./operational-preparation/source-review-supplement/followups/factory-native-overlap-v1/SUMMARY.json)|원래 60초 timer와 RPC callback 겹침|
|[owner-schedule-contract-v1](./operational-preparation/source-review-supplement/followups/owner-schedule-contract-v1/ADOPTION-MANIFEST.json)|사용자가 승인한 실제 4·통제 1 범위|

모든 근거는 AI가 만든 DEV 근거다. 어느 것도 독립 정답이나 독립 승인이 아니다. 13묶음 완료 0/13, 정식 수용 0/9, 독립 승인 0, FINAL 0, freeze 없음, `execution_allowed=false`다.
