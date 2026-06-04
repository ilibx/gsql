create table tbl_logs (
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
  url = 'postgres://dev:dev123@192.168.2.12:5432/onehub?sslmode=disable',
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
  WHERE l."type" = 2  AND l.created_at IS NOT NULL and to_char(to_timestamp(l.created_at), 'YYYY-MM') = '{{ month }}'
  and username in('{{ user|safe }}')
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
ORDER BY year_month DESC, username, channel_name, model_name;`
);

select * from tbl_logs;