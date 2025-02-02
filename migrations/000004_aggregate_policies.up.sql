-- Add refresh policies for continuous aggregates
SELECT add_continuous_aggregate_policy('spot_prices_monthly',
    start_offset => INTERVAL '5 years',
    end_offset => INTERVAL '1 month',
    schedule_interval => INTERVAL '1 month',
    if_not_exists => TRUE
);

SELECT add_continuous_aggregate_policy('spot_prices_daily',
    start_offset => INTERVAL '1 year',
    end_offset => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE
); 