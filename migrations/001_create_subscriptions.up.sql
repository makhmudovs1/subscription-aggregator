CREATE TABLE subscriptions (
   id UUID PRIMARY KEY,
   service_name TEXT NOT NULL,
   price INTEGER NOT NULL,
   user_id UUID NOT NULL,
   start_date DATE NOT NULL,
   end_date DATE
);
