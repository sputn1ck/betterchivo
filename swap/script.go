package swap

import (
	"encoding/hex"
	"github.com/btcsuite/btcd/txscript"
)

// returns the swap script miniscript: or(and(pk(taker),sha256(paymentHash)),and(pk(maker),older(csv)))
func GetSwapScript(makerPkHash []byte, takerPkHash []byte, paymentHash []byte, csv uint32) ([]byte, error ){
	return txscript.NewScriptBuilder().
		AddData(takerPkHash).AddOp(txscript.OP_CHECKSIG).AddOp(txscript.OP_NOTIF).
			AddData(makerPkHash).AddOp(txscript.OP_CHECKSIGVERIFY).AddInt64(int64(csv)).AddOp(txscript.OP_CHECKSEQUENCEVERIFY).
		AddOp(txscript.OP_ELSE).
		AddOp(txscript.OP_SIZE).AddData(h2b("20")).AddOp(txscript.OP_EQUALVERIFY).AddOp(txscript.OP_SHA256).AddData(paymentHash).AddOp(txscript.OP_EQUAL).
		AddOp(txscript.OP_ENDIF).Script()
}

func h2b(str string) []byte {
	buf, _ := hex.DecodeString(str)
	return buf
}