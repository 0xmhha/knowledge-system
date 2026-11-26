// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import { Mid } from "../abstracts/Mid.sol";
import { IToken } from "../interfaces/IAsset.sol";
import { SetLib } from "../libraries/SetLib.sol";

contract Minter is Mid {
    using SetLib for SetLib.Set;

    error ZeroAmount();
    error NotAllowed(address who);

    event Minted(address indexed to, uint256 amount);

    IToken public token;
    SetLib.Set private allowed;
    uint256 public minted;
    mapping(address => uint256) public quota;
    mapping(address => mapping(address => uint256)) internal grants;

    modifier whenNotPaused() {
        require(!paused, AlreadyPaused());
        _;
    }

    modifier notZero(address who) {
        require(who != address(0), NotAllowed(who));
        _;
    }

    constructor(address tokenAddress) {
        token = IToken(tokenAddress);
    }

    /// @notice Mints `amount` to `to`.
    /// @param to receiver
    function mint(address to, uint256 amount) external whenNotPaused notZero(to) notZero(msg.sender) notZero(address(token)) returns (bool) {
        if (amount == 0) revert ZeroAmount();
        if (!allowed.contains(to)) revert NotAllowed(to);
        _record(to, amount);
        bytes32 h = SetLib.hash(to);
        token.mint(to, amount);
        try token.burn(amount) {
            minted -= amount;
        } catch {
            revert ZeroAmount();
        }
        emit Minted(to, amount);
        return h != bytes32(0);
    }

    function allow(address who) external onlyMember {
        allowed.add(who);
        quota[who] = MAX_MEMBERS;
        grants[who][msg.sender] = quota[who];
        delete quota[msg.sender];
    }

    function _record(address to, uint256 amount) private {
        minted += amount;
        proposals[minted].proposer = to;
        proposals[minted].status = Status.Active;
    }

    function _execute(uint256 id) internal override returns (bool) {
        return proposals[id].status == Status.Active;
    }

    function _onMemberAdded(address who) internal override {
        super._onMemberAdded(who);
        quota[who] = 1;
    }

    function _midHook() internal override {
        midValue = minted;
    }

    function withinSupply(uint256 amount) external view returns (bool) {
        return amount <= token.totalSupply();
    }
}
