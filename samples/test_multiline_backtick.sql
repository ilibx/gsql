CREATE TABLE tbl_logs (
	username string,
	channel_name string,
	model_name string,
	year_month string,
	input long,
	completion long,
	quota long,
	cost float,
	requests long
) WITH (
  url = 'postgres://dev:dev123@127.0.0.1:5432/onehub?sslmode=disable',
	query = `WITH tbl_basic AS (
  SELECT
    COALESCE(l.username, '') AS username,
  	c.name as channel_name,
    COALESCE(l.model_name, '') AS model_name,
    to_char(to_timestamp(l.created_at), 'YYYY-MM') AS year_month,
    l.prompt_tokens,
    l.completion_tokens,
    l.quota
  FROM public.logs l left join channels c on l.channel_id = c.id
  WHERE l."type" = 2  AND l.created_at IS NOT NULL
)
SELECT
  username,
  channel_name,
  model_name,
  year_month,
  SUM(prompt_tokens)     AS input,
  SUM(completion_tokens) AS completion,
  SUM(quota)             AS quota,
  ROUND(SUM(quota) / 500000.0, 4) AS cost,
  COUNT(*)               AS requests
FROM tbl_basic
GROUP BY username, channel_name, model_name, year_month
ORDER BY year_month DESC, username, channel_name, model_name;`,
    ssh_host = 'ec2-xxx.com',
	ssh_user = 'ec2-user',
	ssh_key_data = `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA0dBqLNLEIqM0gDekK50t9A3qiP4NZWTMbYyn6fulHfHIFMOO
HOpr+rjl8VIkd/chqgUC1jiqAos81NItQwGFEOVKbXHRYW8FaJeOPieR80DUWrlO
ZZEq0hxmiMIwE7vbCkQUmw9tvxOpZdktJNOn7tU3dI+m/Cq9IA+DpA==
-----END RSA PRIVATE KEY-----`
);

select * from tbl_logs;
