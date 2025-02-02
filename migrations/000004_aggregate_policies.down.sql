-- Drop continuous aggregate policies
SELECT remove_continuous_aggregate_policy('spot_prices_monthly', if_exists => TRUE);
SELECT remove_continuous_aggregate_policy('spot_prices_daily', if_exists => TRUE);
