BEGIN; 

CREATE TABLE users (
  chat_id BIGINT PRIMARY KEY
);

CREATE TABLE links (
  link_id UUID PRIMARY KEY,
  path TEXT NOT NULL UNIQUE,
  resource_type TEXT NOT NULL
);

CREATE TABLE subscriptions (
  subscription_id SERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(chat_id),
  res_id UUID NOT NULL REFERENCES links(link_id),
  link TEXT NOT NULL,
  tags TEXT[] NOT NULL DEFAULT '{}',
  last_update TIMESTAMPTZ,
  UNIQUE (user_id, res_id)
);


CREATE INDEX index_subs_user_id ON subscriptions(user_id);
CREATE INDEX index_subs_res_id ON subscriptions(res_id);

COMMIT;
