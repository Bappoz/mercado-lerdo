-- Schema inicial do Mercado Lerdo.

CREATE TABLE users (
    id            bigserial PRIMARY KEY,
    email         text        NOT NULL UNIQUE,
    name          text        NOT NULL,
    password_hash text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    token      text        PRIMARY KEY,
    user_id    bigint      NOT NULL REFERENCES users (id),
    expires_at timestamptz NOT NULL
);

CREATE TABLE products (
    id          bigserial PRIMARY KEY,
    sku         text        NOT NULL UNIQUE,
    name        text        NOT NULL,
    description text        NOT NULL,
    category    text        NOT NULL,
    price_cents integer     NOT NULL CHECK (price_cents > 0),
    stock       integer     NOT NULL CHECK (stock >= 0),
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE orders (
    id          bigserial PRIMARY KEY,
    user_id     bigint      NOT NULL REFERENCES users (id),
    order_ref   text        NOT NULL,
    status      text        NOT NULL,
    total_cents integer     NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX orders_user_id_idx ON orders (user_id);

CREATE TABLE order_items (
    order_id    bigint  NOT NULL REFERENCES orders (id),
    product_id  bigint  NOT NULL REFERENCES products (id),
    quantity    integer NOT NULL CHECK (quantity > 0),
    price_cents integer NOT NULL,
    PRIMARY KEY (order_id, product_id)
);
