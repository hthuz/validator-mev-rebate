	
	
	# with cross block, only present in mysql
	CREATE TABLE IF NOT EXISTS sandwiches_nobundle(
	slot UInt64,
	timestamp DateTime,
	signature String,
	position Int,
	type String,
	signer String,
	accountKeys Array(String),
	consecutive Bool,
	programs Array(String),
	inbundle Bool,
    signerSame Bool,
    crossBlock Bool,
    oppositeVictim Bool,
	fromToken String,
	toToken String,
	fromAmount Float64,
	toAmount Float64,
	fromDiff Float64,
	toDiff Float64
	) ENGINE = MergeTree()
	 ORDER BY timestamp



	# without cross block.
	CREATE TABLE IF NOT EXISTS sandwiches_nobundle(
	slot UInt64,
	timestamp DateTime,
	signature String,
	position Int,
	type String,
	signer String,
	accountKeys Array(String),
	consecutive Bool,
	programs Array(String),
	inbundle Bool,
	fromToken String,
	toToken String,
	fromAmount Float64,
	toAmount Float64,
	fromDiff Float64,
	toDiff Float64
	) ENGINE = MergeTree()
	 ORDER BY timestamp