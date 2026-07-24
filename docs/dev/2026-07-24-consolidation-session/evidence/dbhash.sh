#!/bin/bash
# 전 테이블 정렬 덤프 해시. FTS 내부 shadow 테이블(_data/_idx/_docsize/_config)과
# sqlite_sequence(자동증가 카운터), manifest의 타임스탬프 행은 비교 의미가 없어 분리 처리.
DB=$1
for t in nodes edges blobs node_prs pending_refs pkg_tree topic_tree; do
  cols=$(sqlite3 "$DB" "SELECT group_concat(name) FROM pragma_table_info('$t')")
  h=$(sqlite3 "$DB" "SELECT * FROM $t ORDER BY $cols" | shasum -a 256 | cut -c1-16)
  n=$(sqlite3 "$DB" "SELECT count(*) FROM $t")
  echo "$t rows=$n hash=$h"
done
# manifest: 시간 의존 키 제외
h=$(sqlite3 "$DB" "SELECT key,value FROM manifest WHERE key NOT IN ('build_timestamp','staleness_mtime_sum') ORDER BY key" | shasum -a 256 | cut -c1-16)
echo "manifest(-timestamp) hash=$h"
# FTS 논리 내용: 가상테이블 자체를 질의(내부 인덱스 표현이 아니라 색인된 텍스트)
h=$(sqlite3 "$DB" "SELECT rowid,name,qualified_name,signature,doc_comment FROM nodes_fts ORDER BY rowid" | shasum -a 256 | cut -c1-16)
echo "nodes_fts(content) hash=$h"
