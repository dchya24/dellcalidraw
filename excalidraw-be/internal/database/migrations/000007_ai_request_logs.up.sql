-- AI Request Logs table (development only)
-- Stores LLM API request/response data for debugging and analysis

CREATE TABLE IF NOT EXISTS ai_request_logs (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Request metadata
    request_id TEXT NOT NULL,
    model TEXT NOT NULL,
    provider TEXT NOT NULL DEFAULT 'openai',
    
    -- Request content
    user_message TEXT NOT NULL,
    system_prompt TEXT,
    canvas_element_count INTEGER NOT NULL DEFAULT 0,
    tools_count INTEGER NOT NULL DEFAULT 0,
    
    -- Response content
    response_text TEXT,
    tool_calls JSONB,
    finish_reason TEXT,
    
    -- Performance metrics
    request_duration_ms INTEGER,
    prompt_tokens INTEGER,
    completion_tokens INTEGER,
    total_tokens INTEGER,
    
    -- Status
    status TEXT NOT NULL DEFAULT 'pending', -- pending, success, error
    error_message TEXT,
    
    -- HTTP metadata
    client_ip TEXT,
    user_agent TEXT
);

-- Indexes for common queries
CREATE INDEX idx_ai_request_logs_created_at ON ai_request_logs (created_at DESC);
CREATE INDEX idx_ai_request_logs_model ON ai_request_logs (model);
CREATE INDEX idx_ai_request_logs_status ON ai_request_logs (status);
CREATE INDEX idx_ai_request_logs_request_id ON ai_request_logs (request_id);

-- Auto-cleanup: keep only last 7 days of logs (run periodically)
-- This is a development tool, no need to keep forever
CREATE OR REPLACE FUNCTION cleanup_ai_request_logs()
RETURNS void AS $$
BEGIN
    DELETE FROM ai_request_logs WHERE created_at < NOW() - INTERVAL '7 days';
END;
$$ LANGUAGE plpgsql;
