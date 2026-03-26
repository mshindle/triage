-- Enable the pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Main messages table for the Signal Group/Channel
CREATE TABLE messages (
                          id SERIAL PRIMARY KEY,
                          signal_id TEXT UNIQUE,
                          sender_phone TEXT NOT NULL,
                          content TEXT NOT NULL,
                          category TEXT,
                          priority INT DEFAULT 50,
                          reasoning TEXT,
                          group_id TEXT, -- To keep messages in the same channel context
                          embedding vector(768), -- Dimensions for Gemini/OpenAI Small (Change to 1536 for OpenAI Large)
                          created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Feedback table to store "Lessons Learned"
CREATE TABLE feedback_memory (
                                 id SERIAL PRIMARY KEY,
                                 original_message_id INT REFERENCES messages(id),
                                 feedback_text TEXT NOT NULL,
                                 embedding vector(768),
                                 created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance
CREATE INDEX ON messages USING hnsw (embedding vector_cosine_ops);
CREATE INDEX ON messages (created_at DESC);
CREATE INDEX ON messages (group_id);