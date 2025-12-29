-- subscription table for user follow relationships
CREATE TABLE subscription (
    id INT AUTO_INCREMENT PRIMARY KEY,
    follower_id INT NOT NULL,
    following_id INT NOT NULL,
    created_ts BIGINT NOT NULL DEFAULT (UNIX_TIMESTAMP()),
    UNIQUE KEY unique_subscription (follower_id, following_id),
    FOREIGN KEY (follower_id) REFERENCES user(id) ON DELETE CASCADE,
    FOREIGN KEY (following_id) REFERENCES user(id) ON DELETE CASCADE
);

CREATE INDEX idx_subscription_follower_id ON subscription(follower_id);
CREATE INDEX idx_subscription_following_id ON subscription(following_id);
