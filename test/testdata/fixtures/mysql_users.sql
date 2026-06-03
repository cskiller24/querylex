-- MySQL fixture: users table with various column types for schema validation tests.
-- This is a small, focused fixture for edge-case and schema validation tests.
-- Real dataset loading goes through load-*.sh scripts in test/scripts/.

CREATE TABLE users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE,
    score DECIMAL(10,2) DEFAULT 0.00
);

INSERT INTO users (username, email, created_at, is_active, score) VALUES
('alice', 'alice@example.com', '2024-01-15 10:30:00', TRUE, 95.50),
('bob', 'bob@example.com', '2024-02-20 14:00:00', TRUE, 82.00),
('charlie', 'charlie@example.com', '2024-03-10 09:15:00', FALSE, 0.00);
