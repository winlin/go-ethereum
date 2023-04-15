#!/bin/sh

# Initialize Geth
geth --datadir "./l2geth-datadir" init "./genesis.json"

# Start Geth with the required parameters
geth \
    --datadir "./l2geth-datadir" \
    --networkid 534353 \
    --bootnodes "enode://996a655365e731321ca35636f5a62fdf37c0b75dc56a8832c472d077da5af47effe45874196268f6083b8f65e1a9589ed25015f68f47598c7bcc93ac8ea29e8a@35.85.116.190:30303" \
    --syncmode full \
    --http --http.port 8545 --http.api "eth,net,web3,debug,scroll" \
    --gcmode archive \
    --verbosity 3