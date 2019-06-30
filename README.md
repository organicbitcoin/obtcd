# Organic Bitcoin - OBTC

*This project is currently working in progress.*

## Prerequisites
- [Bitcoin](https://bitcoin.org/en/)
- [UTXO](https://bitcoin.org/en/glossary/unspent-transaction-output)

## IDEA

OBTC tries to solve some issues existed in the current Bitcoin Network:
- The size of full node keeps increasing.
- Bitcoin can not be retrieved if credentials are lost.
- Dust transactions spammed Bitcoin network.

OBTC introduced one important concept called **Taxation**. For any unspent transaction outputs (UTXOs) that aged longer than 7 years, the OBTC network will consider them as Expired Unspent Transaction Outputs (EUTXOs).  During Proof-of-Work mining, the miner has the privilege to forcely initiate transactions that input from EUTXOs into the coinbase of the new mined block. Here the fee/**taxation** of helping initiate transactions from EUTXOs can be calculated up to a given percentage, in addition, dust EUTXOs can be fully declared.

With the help of Taxation and EUTXOs, Organic Bitcoin will have:
- **Fixed Block Chain Length**: OBTC guarantees that blocks aged longer than 7 years won’t contain any UTXOs.The full node of OBTC does not need to keep blocks aged longer than 7 years. The size of full node is fixed. 
- **Optimize Coin Supply**: Taxation transactions will help release lost coins and dust transactions. It ensures the active coins in the network close to the total coin supply. 
- **Higher Total Block Reward**: Taxation will be rewarded to the miner of new block. The Taxation rate will not be affected by the cut-down of new block reward. The total reward to the miner are consisted of normal transaction fees, new block reward and taxation of EUTXOs.

## Concepts in Organic Bitcoin
- **EUTXO**: Expired Unspent Transaction Output
- **AUTXO**: Active Unspent Transaction Output
- **Dust Amount**: Transaction amount lower than this value is considered as **Dust**. In OBTC, the default value is `5,460`.
- Tax Rate: The charged percentage from EUTXO, by default, it is `30%`.
- **Common Tax Transaction Weight**: The total weight percentage of tax transactions in a new block. Witness data is excluded from total weight calculation. By default, it is `25%`.
- **Urgent Tax Transaction Weight**: The total weight percentage of tax transactions in a new block. If the block height contained the oldest EUTXO is `100` height far lower than the beginning height of the valid block chain, the total weight percentage of tax transactions in the new mined block is raised higher than the common value. Witness data is excluded from total weight calculation. It is `50%` by default.
- **Valid Chain Length**: The valid chain stands for the block chain from the last 7 years. Blocks before 7 years ago can be removed from full node. “7 years” is an approximate value, it is estimated based on new mined block in every 10 minutes. It is `368,208` by default.
- Taxation Begin Height: Currently the OBTC will begin and be hard forked from the height `600,000`.
- **Urgent Expired UTXO Threshold**: The total weight percentage of tax transactions is adjustable. If the block height contained the oldest EUTXO is further away than this threshold from the first block in the valid chain, the total weight will be raised to 50% of the total block weight.
- **zpy**: The minimum OBTC unit.

## Implementation
Organic Bitcoin is implemented in Golang and forked from the project [Btcsuite](https://github.com/btcsuite).

## Roadmap
Organic Bitcoin will be hard forked from Bitcoin main network from the height `600,000`, approximately on `October 1st 2019`.

## Contribution
Any ideas, suggestions and implementations are appreciated!

## Thanks
[Btcsuite](https://github.com/orgs/btcsuite/people) and [Decred](https://github.com/orgs/decred/people) developers.
