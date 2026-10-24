# grpc-go 검색 평가 결과 (1차 실행)

[검색 평가 명세](../../../RETRIEVAL-EVAL-SPEC.md)대로 고정한 입력(PR 100건, 스냅샷 24개)을 두 임베딩 모델로 색인하고, CKS와 비교 대상 네 개를 같은 질문으로 돌린 결과다. 입력은 결과를 본 뒤 바꾸지 않았다. 사람의 정답·질문 표본 확인은 아직이다([표본 확인 자료](../sample-check/PACKET.md)). 그래서 이 수치는 AI가 만든 정답과 질문 위에서 잰 것이다.

## 1. 결과

모든 지표는 PR 100건을 분모로 한다. 오류는 정답 없음으로 센다.

|모델|방식|hit@1|hit@3|MRR|오류|
|---|---|---|---|---|---|
|qwen3-embedding:4b|CKS (`cks eval` 그대로)|0.07|0.11|0.107|0|
|qwen3-embedding:4b|CKS (커밋 필터 없이 다시 채점)|0.14|0.19|0.189|0|
|qwen3-embedding:4b|벡터만|0.24|0.32|0.292|0|
|qwen3-embedding:4b|벡터와 BM25 재정렬|0.18|0.29|0.258|0|
|qwen3-embedding:4b|그래프 낱말 검색|0.02|0.03|0.040|23|
|qwen3-embedding:4b|grep|0.14|0.26|0.201|0|
|bge-m3|CKS (`cks eval` 그대로)|0.03|0.13|0.088|0|
|bge-m3|CKS (커밋 필터 없이 다시 채점)|0.11|0.26|0.209|0|
|bge-m3|벡터만|0.20|0.34|0.291|0|
|bge-m3|벡터와 BM25 재정렬|0.14|0.28|0.241|0|
|bge-m3|그래프 낱말 검색|0.02|0.03|0.040|23|
|bge-m3|grep|0.14|0.26|0.201|0|

그래프 낱말 검색과 grep은 임베딩을 쓰지 않으므로 두 모델에서 같다. 그래프 낱말 검색의 오류 23건은 모두 `question has no usable tokens`다. 한국어로만 된 질문에서 낱말을 하나도 뽑지 못한다.

CKS 의도 정확도는 qwen3-embedding:4b가 0/100, bge-m3가 14/100이다. 대부분 `unknown`이다(qwen3 96건, bge-m3 57건). 혼동표는 `summary.json`에 있다.

응답 크기와 지연은 아래와 같다. 토큰은 글자 수를 4로 나눈 값이고, grep은 파일 경로만 돌려주므로 0이다. 첫 정답까지의 토큰 수는 재지 않았다.

|모델|CKS 평균 토큰|CKS 지연 p50 / p95|벡터만 평균 토큰|
|---|---|---|---|
|qwen3-embedding:4b|872|1,668 / 2,504 ms|1,455|
|bge-m3|1,736|835 / 1,381 ms|3,119|

## 2. CKS 수치를 두 줄로 낸 이유

`cks eval`은 시나리오의 스냅샷 커밋과 다른 커밋이 붙은 인용을 순위를 매기기 전에 뺀다(`internal/system/eval/runner.go`). 두 번째 스냅샷부터는 앞 색인을 복사해 `ckv reindex`로 바뀐 파일만 다시 임베딩했기 때문에, 바뀌지 않은 파일의 청크에는 처음 임베딩한 커밋이 그대로 남는다. 그래서 CKS 인용 대부분이 빠졌다(qwen3 100건 중 84건, bge-m3 84건이 `snapshot_state: conflict`). 비교 대상은 이 필터를 거치지 않으므로, `cks eval` 그대로의 수치는 CKS에 불리하다.

다시 채점한 줄은 녹화한 응답(`--record-responses`)의 인용을 같은 겹침 규칙으로 커밋 필터 없이 다시 순위 매긴 것이다(`scripts/retrieval_eval_summary.py`의 `cks_any_commit`). 이렇게 확인했다.

- 다른 커밋이 붙은 인용(qwen3 604개, bge-m3 590개)의 파일은 모두 그 커밋과 스냅샷 사이에 바뀌지 않았다(`git diff --name-only`로 확인, 바뀐 것 0개). 줄 위치가 스냅샷에서도 그대로라는 뜻이다.
- 필터가 아무것도 빼지 않은 시나리오 32건에서는 다시 채점한 순위가 `cks eval`의 순위와 모두 같다.
- 필터 영향이 없는 첫 스냅샷(16건)만 보면 qwen3에서 CKS hit@1 0.25, 벡터만 0.50, bge-m3에서 CKS 0.0625, 벡터만 0.3125다.

다시 채점해도 CKS는 벡터만 쓴 검색보다 낮다. 이 결과를 보고 검색 로직을 고치는 것은 별도 작업이다.

## 3. 실행 중 고친 것

- **`ckv reindex`가 그래프 커밋을 갱신하지 않던 결함** (`9c7231e8`): `reindex`는 새 그래프에 맞춰 청크를 다시 정렬하고 그래프 digest는 바꾸지만, `sources.ckg.src_commit`은 첫 스냅샷 커밋으로 남겨 두었다. CKS는 이 값과 그래프 커밋이 다르면 모든 요청을 거절한다. 첫 실행에서 CKS 200건 중 168건이 그래프와 벡터 색인의 커밋이 다르다는 `built from different commits` 오류였다. 코드를 고치고 시험을 더했다. 이미 만든 색인 46개는 다시 만들지 않고 이 한 칸만 고쳤다(`index/fix-ckg-ledger.py`, 원본은 각 색인의 `manifest.json.pre-ledger-fix`). 고친 `ckv`로 첫 번째에서 두 번째 스냅샷으로 `reindex`를 다시 해 보니 `vector.db`의 sha256이 같고 이 칸만 달랐다. `sources.code.indexed_head`도 첫 스냅샷 커밋으로 남아 있지만 CKS가 읽지 않아 그대로 두었다.
- **`--record-responses` 누락** (`d43955e4`): 명세는 원응답을 남기라고 하는데 실행 스크립트가 이 옵션을 빠뜨렸다. 더한 뒤 다시 돌렸다.
- **색인 중 바이너리 재빌드**: 색인 도중 `make docs-check`가 `bin/`을 다시 빌드해 해시가 바뀌었다. 색인을 시작한 커밋 `09be8ea2`와 실행 시점 HEAD를 VCS 정보 없이 빌드하면 세 바이너리 모두 바이트가 같다(`index/bin-repro-novcs-sha256.txt`). 평가는 복사해 둔 바이너리(`index/bin-eval-sha256.txt`)로만 돌렸다.

앞선 실행의 출력은 지우지 않고 작업 폴더에 남겼다(`eval-smoke`, `eval-without-responses`, `eval-interrupted-stale-ledger`).

## 4. 색인 시간

|모델|첫 스냅샷 전체 색인|이후 `reindex` 중앙값|벡터 합계|
|---|---|---|---|
|qwen3-embedding:4b|5,547초|231초|3.83시간|
|bge-m3|750초|42초|0.57시간|

그래프는 스냅샷마다 한 번 만들어 두 모델이 같이 쓴다(합계 1.91시간). 평가 실행은 두 모델 합쳐 19분이다.

색인 로그(stderr 포함)에 `embedded from a truncated input`과 `skipping chunk`는 두 모델 모두 0건이다. 2026-10-23 실행에서 `qwen3-embedding:4b`가 16개 청크를 잘랐던 것과 다르다. 그때와 무엇이 달랐는지는 확인하지 않았다. `reindex`된 색인과 전체 색인의 차이는 앞선 비교(`90f571db`, 패키지 convention 요약 청크 15,860개 중 12개)만 있고 이번 실행에서 다시 재지 않았다.

## 5. 아직 판단하지 않은 것

- 의도 분류기의 `unknown` 기준값 0.55는 bge-m3에 맞춘 값이다(`internal/system/composer/intent/classifier.go`). qwen3에서 거의 모두 `unknown`인 것이 이 기준값 때문인지, 한국어 질문과 영어 기준 문장의 거리 때문인지는 재지 않았다.
- 스냅샷 커밋 판정을 어떻게 고칠지(색인을 전체로 다시 만들지, `cks eval`이 바뀌지 않은 파일의 인용을 받아들이게 할지)는 정하지 않았다.
- 그래프 낱말 검색이 한국어 질문을 처리하지 못하는 문제는 고치지 않았다.

## 6. 파일

- `summary.json`: 위 표의 원자료. `scripts/retrieval_eval_summary.py`로 만든다
- `runs/<모델>/<스냅샷>/cks.json`: 녹화한 응답을 포함한 `cks eval` 보고서
- `runs/<모델>/<스냅샷>/baselines.json`: 비교 대상 네 개의 순위와 토큰
- `runs/<모델>/<스냅샷>/cks-exit.txt`: `cks eval` 종료 코드. 스냅샷 충돌이 있으면 1이다
- `index/`: 색인 기록, 바이너리 해시, 색인 원장 수정 스크립트와 그 기록
- `manifest.json`: 위 파일, 도구, 커밋의 해시

다시 집계하는 법:

```sh
python3 scripts/retrieval_eval_summary.py --work <작업 폴더> --repo <grpc-go clone> \
  --scenarios docs/spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/inputs/scenarios \
  --snapshots <작업 폴더>/snapshots.json --model qwen3-embedding:4b --model bge-m3 --out summary.json
```
