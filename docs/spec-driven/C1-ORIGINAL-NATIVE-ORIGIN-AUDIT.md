# 원래 SQLite·musl 소스와 Linux 헤더 출처 감사

2026-10-06 · C1-05 기술 검토 자료. [요약](../../system/eval/b0-knowledge-system/original-native-origin-m2max-2026-10-06/summary.json), [SQLite 결합](../../system/eval/b0-knowledge-system/original-native-origin-m2max-2026-10-06/sqlite-original-binding.json), [헤더 재생](../../system/eval/b0-knowledge-system/original-native-origin-m2max-2026-10-06/header-replay.json), [49개 원자료 해시](../../system/eval/b0-knowledge-system/original-native-origin-m2max-2026-10-06/evidence-manifest.json)를 기록했다. 제품 코드·승인 입력·운영 키는 변경하지 않았다. 품질/공식 verdict는 null, legal review는 pending이다.

## SQLite 원래 C·공개 헤더

고정 `modernc.org/sqlite@v1.54.0`의 공통 상수는 SQLite 3.53.3, 정수 버전 3053003, source ID `2026-06-26 20:14:12 d4c0e51e4aeb96955b99185ab9cde75c339e2c29c3f3f12428d364a10d782c62`다. [SQLite 공식 릴리스](https://sqlite.org/releaselog/3_53_3.html)와 [고정 amalgamation](https://sqlite.org/2026/sqlite-amalgamation-3530300.zip)을 읽고 `sqlite3.c`·`sqlite3.h`의 버전/source ID를 모듈 상수와 대조했다. C의 SHA3-256 `28e484abdaa43630e34040ef6ed92be973a1ad54107803d8af5145b889c23ed7`은 공식 발표값과 일치한다. 압축파일의 관측 SHA256은 `646421e12aac110282ef8cc68f1a62d4bb15fc7b8f09da0b53e29ee690500431`이며 네 원래 파일의 바이트 수/SHA를 원장에 보존했다.

Darwin arm64/Linux arm64/amd64 생성 파일의 첫 줄 명령은 `sqlite3.c` 입력과 libc/libz/libtcl include 디렉터리를 언급한다. 이를 실제 사용된 모든 헤더의 전처리 추적이나 생성기 버전·입력 closure로 해석하지 않는다. 모듈의 Makefile은 per-target 생성물에 `modernc.org/undup@v0.0.5` dedup을 적용한다. 이번에는 원래 C의 신원·공개 헤더와 배포 모듈 연결을 확인했으며 전체 C→Go 재생성 바이트 일치를 주장하지 않는다.

## musl 고정 커밋과 헤더

`modernc.org/libc@v1.74.1`의 builder.json과 다운로드 파일명 상수는 [musl 커밋 7ada6dde](https://git.musl-libc.org/cgit/musl/commit/?id=7ada6dde6f9dc6a2836c3d92c2f762d35fd229e0)의 압축파일를 지정한다. 해당 공식 서버 원문을 읽어 관측 SHA256 `b65b3c7ca604a839ecb5abcdb7454e2e7d11db2b8e08df668c7fee45e8e50fbf`와 C/헤더/템플릿/COPYRIGHT 2,337항목의 SHA를 기록했다. commit 페이지의 tree 참조는 기록하지만 Git tree 객체 재구성·공식 압축파일 서명/공개 checksum 검증을 수행한 것으로 표현하지 않는다.

| 대상 | 원래 헤더와 바이트 동일 | 원래 생성 명령 재생 | 모듈 overlay 동일 | 전체 |
|---|---:|---:|---:|---:|
| Linux arm64 | 215 | 2 | 1 | 218 |
| Linux amd64 | 215 | 2 | 1 | 218 |

`bits/alltypes.h`는 고정 musl의 `mkalltypes.sed`·arch/include 입력으로, `bits/syscall.h`는 원래 입력 복사와 `__NR_`→`SYS_` 행 추가 명령으로 재생했다. 네 결과가 배포 모듈 헤더와 정확히 같다. 외부 Makefile 전체나 C 코드를 실행하지 않았으며 좁은 sed 명령과 파일 입력만 사용했다. `bits/float.h` 두 개는 고정 모듈의 overlay와 같다. non-ccgo 분기는 원래 텍스트를 보존하고 ccgo 분기는 long double 관련 상수를 바꾼다. 이 변경의 의미·ABI·생성기 전체 정확성을 새로 판정하지 않았다.

generator.go는 원래 musl 소스 일부 삭제·overlay·configure·ccgo 번역·후처리와 헤더 복사를 수행한다. 헤더 디렉터리의 출처를 분류한 것이 모든 실제 include-use나 전체 번역 결과의 재현 증명은 아니다. 배포 모듈에 Darwin arm64 입력 헤더 디렉터리가 없어 해당 SDK 헤더의 원래 바이트·고지를 확정하지 않았다.

## 모듈 신원·검증 한계

두 모듈 ZIP의 h1을 이름/내용에서 다시 계산해 go.sum과 cache ziphash에 대조했다. 버전 상수·생성 명령·recipe·두 Linux 헤더 디렉터리·overlay·고지의 선택된 501파일은 ZIP과 같다. 실행 후 이 입력들의 SHA와 실제 사람 입력5개도 HEAD와 동일하다. 이는 고정 GitLab blob 직접 비교나 모든 모듈/전이 native의 적합성 판정이 아니다.

검증 helper의 첫 overlay 검사에서 래퍼의 빈 줄을 누락한 가정, 수정 스크립트 문법 오류, 수정 실패 뒤 기존 출력 디렉터리 재사용 오류를 보존했다. 수정된 helper는 별도 디렉터리에서 생성4/overlay2의 여섯 검사를 통과했다. 첫 부분 실행과 성공 재생의 분모를 구분하며 제품 실패나 법무 통과로 합산하지 않는다. 원문의 공백은 lossless JSON wrapper로 보관한다. DB/바이너리/모델/키·다운로드 압축파일·큰 원래 C 소스는 Git에 넣지 않았다.

현재 C1-05의 원래 소스·Linux 헤더 출처 검토 자료는 보완됐지만 C1-05는 진행 상태다. 다음 공식 작업은 실제 B0-01/02/03/05·STV2 결정 → 입력 동결 → B0-07 strict 빌드다. 원래 Darwin SDK·generator/include-use/전체 C→Go 재생성·전이 native 범위·OP-06 사람 적합성 판정, OP-01–08 운영 결정, native amd64 실측과 최종 복구/지원/출시는 남는다. 전체 완료2/진행11/대기17·미완료28·공식 게이트0/4이며 [현황판](./EXECUTION-STATUS.md)의 전체 남은 작업을 유지한다.
