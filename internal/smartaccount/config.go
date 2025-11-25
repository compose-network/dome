package smartaccount

// stage only
var (
	kernelFactoryAddress = "0xcfb519af7e3e4b772c619ed12bcdc7d758ac6ee6"
	kernelAbi            = `[{"type":"function","name":"createAccount","stateMutability":"payable","inputs":[{"name":"data","type":"bytes"},{"name":"salt","type":"bytes32"}],"outputs":[{"name":"","type":"address"}]},{"type":"function","name":"getAddress","stateMutability":"view","inputs":[{"name":"data","type":"bytes"},{"name":"salt","type":"bytes32"}],"outputs":[{"name":"","type":"address"}]}]`
	entryPointAddress    = "0x0000000071727De22E5E9d8BAf0edAc6f37da032"
	entryPointABI        = `[{"type":"function","name":"handleOps","stateMutability":"nonpayable","inputs":[{"name":"ops","type":"tuple[]","components":[{"name":"sender","type":"address"},{"name":"nonce","type":"uint256"},{"name":"initCode","type":"bytes"},{"name":"callData","type":"bytes"},{"name":"accountGasLimits","type":"bytes32"},{"name":"preVerificationGas","type":"uint256"},{"name":"gasFees","type":"bytes32"},{"name":"paymasterAndData","type":"bytes"},{"name":"signature","type":"bytes"}]},{"name":"beneficiary","type":"address"}],"outputs":[]},{"type":"function","name":"getAccount","stateMutability":"view","inputs":[{"name":"account","type":"address"}],"outputs":[{"name":"","type":"address"}]}]`
)
