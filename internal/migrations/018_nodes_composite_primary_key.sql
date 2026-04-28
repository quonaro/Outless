-- Change primary key to composite (url, group_id)
-- This allows same URL in different groups
ALTER TABLE nodes DROP CONSTRAINT nodes_pkey;
ALTER TABLE nodes ADD PRIMARY KEY (url, group_id);
