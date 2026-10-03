# Knowledge System 설치 시범 절차

현재 산출물은 **빌드한 호스트에서 검증하는 개발 프리뷰**다. 지정된 대상은 macOS arm64, Linux arm64, Linux amd64다. 운영 배포 전에 각 대상에서 네이티브 라이브러리, 모델, 라이선스와 전체 WBS A7/A8을 확인한다. `mock` 임베더는 구조 스모크 전용이며 자연어 검색 품질을 검증하지 않는다.

## 1. 호스트 패키지

저장소에서 다음을 실행한다.

```sh
python3 scripts/package-host.py --out-dir /tmp/knowledge-system-dist
```

출력 JSON의 `sha256`으로 압축 파일을 확인한다. 신뢰 가능한 공개 키가 별도로 전달된 경우, **해제와 바이너리 실행 전에** 아래 1.1절의 독립 검증기를 사용한다. 압축 파일에는 `ckg`, `ckv`, `cks`, `LICENSE`, `INSTALLATION.md`, 소독 규칙, `modules.txt`, `manifest.json`, `third-party-licenses.json`과 캐시에 존재하는 제3자 라이선스·NOTICE 원문이 들어 있다. `manifest.json`에는 바이너리별 SHA-256·크기, 빌드 커밋, 호스트, Go 버전, 감지한 네이티브 의존성이 기록된다. 작업 트리가 더러울 때 프리뷰가 필요하면 `--allow-dirty`를 명시하며, 매니페스트가 `dirty: true`로 표시된다. 배포 가능 판정은 아니다.

현재 호스트에서 압축 파일 자체를 확인하려면 `scripts/wbs-package-smoke.sh`를 실행한다. 아카이브와 내부 바이너리의 해시를 검증하고, 압축 해제한 바이너리·소독 규칙만 사용해 아래 세 유형의 독립 프로젝트에서 설치와 CKS MCP 질의를 실행한다. 패키지 빌드에는 Go가 필요하지만, 세 가지 문서/TypeScript 설치 스모크는 추출한 바이너리와 Git·Python으로 실행할 수 있다. Go 소스 분석에는 별도 Go 도구 체인이 필요하다. mock 임베더의 검색 품질 판정은 아니다.

Linux arm64의 CGO 빌드 이미지는 `scripts/Dockerfile.linux-build`로 고정한다. 기본 Go 이미지에는 `sqlite3.h`가 없어 CKV/CKS의 sqlite-vec 바인딩을 컴파일할 수 없었고, `libsqlite3-dev`를 더한 이미지에서는 세 바이너리의 빌드·기동 및 독립 프로젝트 세 곳의 설치·MCP 질의가 통과했다. `scripts/wbs-linux-package-smoke.sh arm64|amd64`는 각 대상 이미지에서 압축 패키지를 만들고, 시험 서명·독립 검증 뒤 Go 컴파일러가 없는 Debian 런타임에서 설치·질의·MCP 스모크를 실행한다. 합성 Git 커밋을 사용하는 개발 스모크이므로 운영 릴리스 판정은 아니다.

### 1.1 시험 서명과 최초 설치 전 검증

`scripts/release-sidecar.py`는 기존 `host-preview` 아카이브에 대해 `release.json`과 raw Ed25519 `release.json.sig`를 **아카이브 밖에** 생성한다. 서명의 `scope`는 `test-signed-preview`다. 별도 경로에서 받은 공개 키와 독립적인 `scripts/verify-release.py`로 서명, 아카이브 전체 SHA-256, 대상 OS/CPU, 내부 바이너리·라이선스 원문과 목록·모듈 목록의 해시와 안전한 tar 구조를 검증한 뒤에만 해제한다. 설치 후에는 같은 입력으로 `cks package verify`를 실행할 수 있다. 제3자 라이선스 목록은 자동 적합성 판정이 아니다. 현재 연결 모듈과 로컬 grammar·Tree-sitter runtime/Unicode·Go toolchain 고지를 원문 해시로 기록한다. 누락은 `missing_source_license` 및 manifest의 missing 값에 남기며, 누락 0도 전체 적합성 승인은 아니다. 이전 v0.1.6 캐시의 누락 기록과 현재 연결된 sqlite-vec pseudo-version의 MIT/Apache 원문 수집을 구분한다. 실제 배포 키와 배포 판정은 C1에서 결정한다.

```sh
python3 scripts/verify-release.py \
  --public-key /trusted/release-public.pem \
  --release /download/release.json --signature /download/release.json.sig \
  --archive /download/knowledge-system-darwin-arm64-COMMIT.tar.gz \
  --target-os darwin --target-arch arm64
```

`scripts/wbs-release-sign-smoke.sh`는 임시 시험 키로 정상 서명을 만들고, 다른 키·변조된 사이드카·변조되거나 다른 아카이브·대상 불일치를 각각 거부한다. 독립 검증을 통과한 뒤에만 압축 파일의 `cks package verify`를 실행한다. 이 시험은 프리뷰 서명 프로토콜의 구조 검증이며 배포용 공개 키 신뢰 경로나 타깃별 인증을 대신하지 않는다.

## 2. 프로젝트별 초기화

압축을 푼 디렉터리에서 **다른 Git 저장소**에도 동일한 절차를 적용한다. 데이터셋은 소스 트리 밖에 둔다.

```sh
./cks doctor --src /path/to/project
./cks init --src /path/to/project \
  --dataset /path/to/project-dataset \
  --config-out /path/to/project-setup.yaml \
  --embedder ollama --model-name YOUR_EMBEDDING_MODEL
./cks setup --config /path/to/project-setup.yaml --version auto
./cks doctor --src /path/to/project --dataset /path/to/project-dataset --strict
```

`init`과 `mcp gen-config`는 기존 사용자 설정을 덮어쓰지 않는다. 새 경로를 지정하거나 기존 파일을 명시적으로 이동한 뒤 다시 생성한다. 실제 임베더를 명시해야 하며 Ollama 모델은 별도로 준비해야 한다. `doctor`는 Go, TypeScript, JavaScript, Solidity, Markdown 파일 수와 graph-only Proto 수를 미리 보고한다. 심볼릭 링크와 의심스러운 비밀 경로를 개수로만 보고하며 내용을 읽거나 출력하지 않는다. 이 진단은 프로젝트의 `.ckvignore`와 파일 목록 정책이 제대로 적용됐는지 대체하지 않는다.

Go 소스가 없는 모듈이나 지원하지 않는 Python 코드와 Markdown만 있는 저장소에서도 문서 검색은 가능하지만 코드 AST 그래프와 CKV 코드 심볼의 조인은 없다. `doctor`는 `shared_code_files=0`, `status=degraded`로 이 범위를 표시한다. TypeScript 저장소는 `shared_code_files`를 양수로 보고한다. 세 유형의 독립 저장소에서 `init → setup → doctor → ckv query → CKS MCP context.get_for_task → 새 버전 setup → rollback → query`를 실행하는 구조 스모크는 `scripts/wbs-install-smoke.sh`에 있다. `doctor.commit`은 현재 작업 트리의 HEAD이고 `doctor.indexed_commit`은 활성 pinned 데이터셋의 기반 커밋이다. 롤백 후 작업 트리가 새 커밋이면 둘이 다른 것이 정상이며 과거 데이터셋 인용은 `indexed_commit`에 맞아야 한다. 각 프로젝트의 MCP 서버를 새로 띄워 다시 조회했을 때도 인용 커밋이 그대로였고, TypeScript 코드 질문은 `main.ts`를 인용했다. macOS arm64 및 Linux arm64/amd64의 압축 배포물에서 동일한 스모크를 실행한다. 검색 품질은 mock 임베더이므로 측정하지 않는다.

`--version auto`는 커밋 기반 버전 이름을 사용한다. `committed` 모드의 모든 `setup` 빌드는 추적·미추적 파일이 남아 있거나 Git이 무시하지만 CKV/CKG가 읽을 수 있는 소스 파일이 있으면 거부한다. 명시적 버전 이름을 줘도 예외가 아니다. 버전 후보는 활성화 직전에 빌드 결과의 커밋과 소스 청결성을 다시 검사한다. `--source-mode working-tree`와 `snapshot-only`는 명시적 버전·프로젝트 ID와 보관 원문 캡처를 요구한다. 제한·검증 현황은 `SOURCE-SNAPSHOT-ADR.md`와 `EXECUTION.md`를 따른다. 설정 파일을 소스 저장소 안에 만들었다면 커밋하거나 무시 정책을 정한 뒤 빌드한다.

검토 완료된 CKS 온톨로지·명세를 CKV 자연어 코퍼스에도 넣으려면 먼저 활성 의미 투영을 `cks semantic export-text ... --out /path/to/new-corpus`로 내보내고, 다음 재색인에 `--semantic-corpus /path/to/new-corpus`를 지정한다. 출력 디렉터리는 소스 저장소 밖에 둔다. 후보 그래프와 코퍼스의 저장소·커밋·파일 해시가 맞아야 CKV가 이를 임베딩한다. `--version auto`는 이 코퍼스 매니페스트 해시도 버전 이름에 반영한다. 코퍼스 출처가 달라지면 같은 소스 커밋이라도 새 데이터셋으로 만든다.

## 3. 서비스와 롤백

데이터셋이 승격된 다음 CKS 런타임 설정을 생성한다. 배포용 소독 규칙 경로를 명시한다.

```sh
./cks mcp gen-config --dataset-dir /path/to/project-dataset/current \
  --source-root /path/to/project \
  --sanitize-rules /path/to/unpacked/policies/sanitization_rules.yaml \
  --embed-model YOUR_EMBEDDING_MODEL --out /path/to/cks.yaml
./cks mcp --config /path/to/cks.yaml
```

롤백은 이전에 검증된 버전으로 `current`를 되돌린다. 실행 중인 MCP 프로세스는 이미 연 데이터셋 핸들을 유지하므로 롤백·승격 후 재시작해야 한다.

커밋된 패치에 대해 승격 전 테스트 게이트를 걸려면 `setup --version <new-version> --gate-test-bin go --gate-test-arg test --gate-test-arg ./...`처럼 실행한다. 인자는 각각 전달하며 셸을 거치지 않는다. 빌드한 후보의 `test-gate.json`에는 커밋, 그래프 다이제스트, 명령·출력 해시, 종료 코드와 실행 환경이 남는다. 테스트 실패 또는 실행 전후 소스 변경은 `current`를 바꾸지 않는다. 이는 선택한 **명령의 성공**을 확인하는 게이트이며, 개별 수용 기준의 의미적 충족은 검토자가 판정해야 한다.

```sh
./cks setup --config /path/to/project-setup.yaml --rollback PREVIOUS_VERSION
```

`cks index`는 `cks setup`, `cks status`는 `cks doctor`, `cks serve`는 `cks mcp`의 호환 별칭이다. `cks rollback PREVIOUS_VERSION`은 설정을 지정하면 같은 롤백 검증을 호출한다.

이 절차는 프로젝트별 수동 스모크의 출발점이다. macOS arm64와 Linux arm64/amd64의 시험 서명 압축 배포물은 깨끗한 추출 런타임에서 설치·질의·롤백을 통과했다. 운영 릴리스 키와 제3자 라이선스의 법적 판정은 C1, 실제 임베딩 모델의 품질·지연은 B/C에 남아 있다. 현재 호스트의 구조 스모크는 세 프로젝트를 독립 데이터셋으로 설치해 각 인용의 커밋을 검사했다. 또 다른 스모크에서는 실패 후보가 `current`를 바꾸지 않고, 실행 중인 MCP 프로세스가 교체 전 커밋의 인용을 계속 내보내며, 새 프로세스는 교체 후 커밋을 인용하고, 롤백 후 새 프로세스는 이전 커밋을 다시 인용하는 것을 확인했다.
