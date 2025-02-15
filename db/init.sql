CREATE TABLE users (
    user_id UUID PRIMARY KEY,
    login VARCHAR(32) NOT NULL UNIQUE,
    hash_password VARCHAR(72) NOT NULL,
    amount_in_wallet INTEGER NOT NULL
);

CREATE TABLE store (
    "type" INTEGER NOT NULL UNIQUE,
    price INTEGER NOT NULL
);

CREATE TABLE sessions (
    session_id UUID PRIMARY KEY,
    user_id UUID REFERENCES users(user_id) ON DELETE CASCADE,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL
);

CREATE TABLE items (
    user_id UUID REFERENCES users(user_id) ON DELETE CASCADE,
    "type" INTEGER NOT NULL,
    quantity INTEGER NOT NULL
);

CREATE TABLE transactions (
    trans_id SERIAL PRIMARY KEY,
    sender UUID REFERENCES users(user_id), -- от кого
    receiver UUID REFERENCES users(user_id), -- кому
    amount INTEGER NOT NULL -- сколько
);

INSERT INTO store ("type", price) VALUES
    (0, 80),  -- t-shirt
    (1, 20),  -- cup
    (2, 50),  -- book
    (3, 10),  -- pen
    (4, 200), -- powerbank
    (5, 300), -- hoody
    (6, 200), -- umbrella
    (7, 10),  -- socks
    (8, 50),  -- wallet
    (9, 500); -- pink-hoody
