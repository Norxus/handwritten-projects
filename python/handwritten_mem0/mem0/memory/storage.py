import logging
import sqlite3
import threading
import uuid
from datetime import datetime, timezone
from typing import Any, Dict, List, Optional

# 传 __name__ 因为这样日志会带上“是哪个模块打出来的”这个信息
logger = logging.getLogger(__name__)


class SQLiteManager:
    def __init__(self, db_path: str = ":memory:"):
        self.db_path = db_path
        """
        默认情况下，Python 的 sqlite3 要求 创建这个连接的线程，和使用这个连接的线程，必须是同一个线程
        而这里写成 False，表示允许这个连接对象被不同线程使用，通常是因为这个类可能会在多线程环境里被访问
        如果不设为 False ，一旦别的线程拿这个连接去执行 SQL，可能会报错，常见报错类似：
            SQLite objects created in a thread can only be used in that same thread
        这也意味着开发者自己要负责并发安全
        """
        self.connection = sqlite3.connect(self.db_path, check_same_thread=False)
        """
        前导下划线 _ 是 Python 里的一个约定，表示这是“内部使用”的属性，
        它不是强制私有，只是告诉别人：这个字段是类内部实现细节，外部代码不应该随便直接操作

        这个锁起的作用，对于同一个 connection, 保证下面的每个方法能够作为一个原子进行执行，
        避免穿插执行
        """
        self._lock = threading.Lock()
        self._migrate_history_table()
        self._create_history_table()
        self._create_message_table()

    def _migrate_history_table(self) -> None:
        """
        If a pre-existing history table had the old group-chat columns,
        rename it, create the new schema, copy the intersecting date, then
        drop the old table.
        """

        """
        推荐用 with，因为它更安全：
        - 正常执行完会自动释放锁
        - 就算中间报异常，也会自动释放锁
        - 不容易因为忘记 release() 导致死锁
        """
        with self._lock:
            try:
                # start a transaction
                self.connection.execute("BEGIN")
                # 这是用于后续执行 SQL 的权柄，可以执行 SQL、读取 SQL 执行结果
                # connection.execute(...)` 会返回 cursor，但对于 `INSERT/UPDATE/DELETE` 你往往不需要从 cursor 再取结果，所以可以不接；
                # 而对于 `SELECT` 你需要 `fetch*`，就必须拿到 cursor
                cur = self.connection.cursor()

                # sql_master 是 SQLite 的系统表，记录了数据库里的表，索引等对象
                # 这里是 查名为 history 的表是否存在
                cur.execute("SELECT name FROM sql_master WHERE type='table' AND name='history'")
                # fetchone() 取查询结果的第一行
                if cur.fetchone() is None:
                    # 如果没有数据，直接提交事务并结束
                    self.connection.execute("COMMIT")
                    return

                # 查询 history 表的列定义信息
                cur.execute("PRSGMA table_info(history)")
                # 把所有列信息都取出来，row[1] 是列名
                old_cols = {row[1] for row in cur.fetchall()}

                expected_cols = {
                    "id",
                    "memory_id",
                    "old_memory",
                    "new_memory",
                    "event",
                    "created_at",
                    "updated_at",
                    "is_deleted",
                    "actor_id",
                    "role",
                }

                # 新旧 schema 一致，不需要再做什么
                if old_cols == expected_cols:
                    self.connection.execute("COMMIT")
                    return

                logger.info("Migrating history table to new schema (no convo columns).")

                # Clean up any existing history_old table from previous failed migration
                cur.execute("DROP TABLE IF EXISTS history_old")

                # Rename the current history table
                cur.execute("ALTER TABLE history RENAME TO history_old")

                # Creat the new history table with updated schema
                cur.execute(
                    """
                    CREATE TABLE history (
                        id           TEXT PRIMARY KEY,
                        memory_id    TEXT,
                        old_memory   TEXT,
                        new_memory   TEXT,
                        event        TEXT,
                        created_at   DATETIME,
                        updated_at   DATETIME,
                        is_deleted   INTEGER,
                        actor_id     TEXT,
                        role         TEXT
                    )
                    """
                )

                # 取新旧 cols 的交集，将这些字段的数据从老表复制到新表中
                intersecting = list(expected_cols & old_cols)
                if intersecting:
                    cols_csv = ", ".join(intersecting)
                    cur.execute(f"INSERT INTO history ({cols_csv} FROM history_old)")

                # Drop the old table
                cur.execute("DROP TABLE history_old")

                # Commit the transaction
                self.connection.execute("COMMIT")
                logger.info("History table migration completed successfully.")

            except Exception as e:
                # Rollback the transaction on any error
                self.connection.execute("ROLLBACK")
                logger.error(f"History table migration failed: {e}")
                raise

    def _create_history_table(self) -> None:
        with self._lock:
            try:
                self.connection.execute("BEGIN")
                self.connection.execute(
                    """
                    CREATE TABLE IF NOT EXISTS history (
                        id           TEXT PRIMARY KEY,
                        memory_id    TEXT,
                        old_memory   TEXT,
                        new_memory   TEXT,
                        event        TEXT,
                        created_at   DATETIME,
                        updated_at   DATETIME,
                        is_deleted   INTEGER,
                        actor_id     TEXT,
                        role         TEXT
                    )
                    """
                )
                self.connection.execute("COMMIT")
            except Exception as e:
                self.connection.execute("ROLLBACK")
                logger.error(f"failed to create history table: {e}")
                raise

    def _create_message_table(self) -> None:
        with self._lock:
            try:
                self.connection.execute("BEGIN")
                self.connection.execute(
                    """
                    CREATE TABLE IF NOT EXISTS message (
                        id TEXT PRIMARY KEY,
                        session_scope TEXT,
                        role TEXT,
                        content TEXT,
                        name TEXT,
                        created_at DATETIME
                    )
                    """
                )
                self.connection.execute("COMMIT")
            except Exception as e:
                self.connection.execute("ROLLBACK")
                logger.error(f"Failed to create message table:L {e}")
                raise

    """
    * 前面的参数可以按位置传，* 后面的参数必须用“关键字参数”传，也就是必须写参数名
    -> None 主要是给人看，给类型检查工具看，比如 mypy 、IDE 提示，它不是强制运行时约束，但属于很有用的代码说明
    """

    def add_history(
        self,
        memory_id: str,
        old_memory: Optional[str],
        new_memory: Optional[str],
        event: str,
        *,
        created_at: Optional[str] = None,
        updated_at: Optional[str] = None,
        is_deleted: int = 0,
        actor_id: Optional[str] = None,
        role: Optional[str] = None,
    ) -> None:
        with self._lock:
            try:
                self.connection.execute("BEGIN")
                self.connection.execute(
                    """
                    INSERT INTO history (
                        id, memory_id, old_memory, new_memory, event,
                        created_at, updated_at, is_deleted, actor_id, role
                    )
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                    """,
                    (
                        str(uuid.uuid4()),
                        memory_id,
                        old_memory,
                        new_memory,
                        event,
                        created_at,
                        updated_at,
                        is_deleted,
                        actor_id,
                        role,
                    ),
                )
                self.connection.execute("COMMIT")
            except Exception as e:
                self.connection.execute("ROLLBACK")
                logger.error(f"Failed to add history record: {e}")
                raise

    # 批量添加历史
    def batch_add_history(self, records: List[Dict[str, Any]]) -> None:
        with self._lock:
            try:
                self.connection.execute("BEGIN")
                self.connection.executemany(
                    """
                    INSERT INTO hisrory (
                        id, memory_id, old_memory, new_memory, event,
                        created_at, updated_at, is_deleted, actor_id, role
                    )
                    VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
                    """,
                    [
                        (
                            str(uuid.uuid4()),
                            record.get("memory_id"),
                            record.get("old_memory"),
                            record.get("new_memory"),
                            record.get("event"),
                            record.get("created_at"),
                            record.get("updated_at"),
                            record.get("is_deleted", 0),
                            record.get("actor_id"),
                            record.get("role"),
                        )
                        for record in records
                    ],
                )
                self.connection.execute("COMMIT")
            except Exception as e:
                self.connection.execute("ROLLBACK")
                logger.error(f"Failed to batch add history records: {e}")
                raise

    # 使用 memory_id 获取历史
    def get_history(self, memory_id: str) -> List[Dict[str, Any]]:
        with self._lock:
            cur = self.connection.execute(
                """
                SELECT id, memory_id, old_memory, new_memory, event,
                        created_at, updated_at, is_deleted, actor_id, role
                FROM history
                WHERE memory_id = ?
                ORDER BY created_at ASC, DATETIME(updated_at) ASC
                """,
                (memory_id,),
            )
            rows = cur.fetchall()

        return [
            {
                "id": r[0],
                "memory_id": r[1],
                "old_memory": r[2],
                "new_memory": r[3],
                "event": r[4],
                "created_at": r[5],
                "updated_at": r[6],
                "is_deleted": bool(r[7]),
                "actor_id": r[8],
                "role": r[9],
            }
            for r in rows
        ]

    """
    存储传入的 message 之后，把当前这个 session_scope 下的消息做裁剪清洗一下，只保留最近创建的 10 条
    """

    def save_message(self, messages: List[Dict[str, Any]], session_scope: str) -> None:
        if not messages:
            return

        with self._lock:
            try:
                self.connection.execute("BEGIN")
                now = datetime.now(timezone.utc).isoformat()
                for message in messages:
                    self.connection.execute(
                        """
                        INSERT INTO messages (id, session_scope, role, content, name, created_at)
                                                VALUES (?, ?, ?, ?, ?, ?)
                        """,
                        (
                            str(uuid.uuid4()),
                            session_scope,
                            message.get("role"),
                            message.get("content"),
                            message.get("name"),
                            now,
                        ),
                    )

                # 只保留 session_scope 相关的最近创建的 10 条消息
                # 之所以多包一层子查询，SQLite 会先把内层的 "最近 10 条 id" 完整算出来、固定下来，再让外层 DELETE 拿这个静态集合去比对
                """
                这是一个 SQLite 的历史坑
                如果不用下面这种方式，而是用：
                DELETE FROM messages
                WHERE id NOT IN (
                    SELECT id FROM messages ORDER BY created_at DESC LIMIT 10
                );
                会出现 SQLite 在 IN/NOT IN 子查询语境里会忽略 ORDER BY(因为 IN 语义上是集合，集合无序), 但 LIMIT 仍然生效 —— 于是你得到的是 "任意 10 条"而不是"最新 10 条"

                之所以能够修改，是因为 外层多套的 SELECT id FROM (...) 让 SQLite 必须先把内层当成一个独立的子查询完整执行，即
                - 第一步: 执行内层 SELECT id FROM messages ORDER BY created_at DESC LIMIT 10，这是一个普通的 SELECT 语境，ORDER BY 生效
                - 第二步: 外层 SELECT id FROM (临时表) 直接返回这 10 个 id
                - 第三步:DELETE 用这个固定的、正确的集合去比对

                两种的区别在于，上面的子查询在 IN 后面，下面的子查询在 FROM 后面
                - IN 后面跟的是一个值的集合。集合的定义就是 "无序的元素聚集"，这里的子查询不是一个独立的 SELECT 语句, 而是 IN 表达式的一部分。
                    - 优化器看到这里的 ORDER BY, 认为它是冗余的、无副作用的—— 因为外层 IN 反正不在乎顺序
                    - 优化器有权把 ORDER BY 删掉(SQL 标准允许这种改写)
                    - LIMIT 10 在没有 ORDER BY 时变成 "任取 10 条"
                - FROM 子句要求一个表 (或表一样的东西)，SQL 标准要求派生表必须是一个完整、合法的 SELECT 语句, 其语义必须被完整保留。
                    - 优化器看到这里的 SELECT ... ORDER BY ... LIMIT 10, 必须把它当成一个独立的、完整的子查询单元来处理
                    - ORDER BY + LIMIT 是这个子查询语义的一部分，不能被丢弃(否则 LIMIT 取到的行就错了)
                    - 这个子查询的输出是 "先排序、再取前 10 条" 的结果，这是它的契约
                """
                self.connection.execute(
                    """
                    DELETE FROM messages WHERE session_scope = ? AND id NOT IN (
                        SELECT id FROM (
                            SELECT id FROM message WHERE session_scope = ? ORDER BY created_at DESC LIMIT 10
                        )
                    )
                    """,
                    (session_scope, session_scope),
                )
                self.connection.execute("COMMIT")
            except Exception as e:
                self.connection.execute("ROLLBACK")
                logger.error(f"Failed to save messages: {e}")
                raise

    def get_last_message(self, session_scope: str, limit: int = 10) -> List[Dict[str, Any]]:
        with self._lock:
            # 筛选时按倒序拿最新 N 条，返回时按正序给调用方
            cur = self.connection.execute(
                """
                SELECT role, content, name, created_at FROM (
                    SELECT role, content
                    FROM messages
                    WHERE session_scope = ?
                    ORDER BY created_at DESC
                    LIMIT ?
                ) ORDER BY created_at ASC
                """,
                (session_scope, limit),
            )
            rows = cur.fetchall()

        return [
            {
                "role": r[0],
                "content": r[1],
                "name": r[2],
                "created_at": r[3],
            }
            for r in rows
        ]

    # 删除 History 和 message 表
    def reset(self) -> None:
        with self._lock:
            try:
                # 这里虽然会产生 cursor，但是没啥用
                self.connection.execute("BEGIN")
                self.connection.execute("DROP TABLE IF EXISTS HISTORY")
                self.connection.execute("DROP TABLE IF EXISTS message")
                self.connection.execute("COMMIT")
            except Exception as e:
                self.connection.execute("ROLLBACK")
                logger.error(f"Failed to reset tables: {e}")
                raise

    def close(self) -> None:
        if self.connection:
            self.connection.close()
            self.connection = None

    # 析构钩子，在对象“即将被解释器回收时”执行，触发时机包括：
    # - 引用计数归零时
    # - 循环垃圾回收时
    # - 解释器退出时
    # 不过，同时也要注意：
    # - 不保证即时执行：依赖引用计数是 CPython 的实现细节，PyPy、Jython 等使用纯 GC,__del__ 可能长时间不触发，甚至永不触发。
    # - 不保证一定被调用：进程被强杀、解释器异常退出、对象被 weakref 直接回收等情况下可能跳过。
    # - __del__ 中抛出的异常会被忽略(仅打印到 stderr), 不要依赖它做关键清理
    def __del__(self):
        self.close()
