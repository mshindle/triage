ALTER TABLE messages ADD COLUMN triage_status TEXT NOT NULL DEFAULT 'pending';
CREATE INDEX ON feedback_memory USING hnsw (embedding vector_cosine_ops);
CREATE TABLE replies (
    id SERIAL PRIMARY KEY,
    original_message_id INT REFERENCES messages(id),
    content TEXT NOT NULL,
    delivery_status TEXT NOT NULL DEFAULT 'pending',
    error_detail TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
