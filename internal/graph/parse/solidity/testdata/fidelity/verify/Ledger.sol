// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {
    Rules,
    Caps,
    IOracle as Feed
} from "./Rules.sol";

contract Ledger {
    using Rules for Rules.Limits;

    enum Stage {
        Idle,
        Live
    }

    struct Entry {
        address owner;
        uint256 amount;
        Stage stage;
    }

    /**
     * @dev Bounds every amount must stay in.
     */

    Rules.Limits private bounds;
    Caps.Limits private ceiling; // a plain comment is no doc
    Feed public oracle;
    mapping(uint256 => Entry) internal entries;
    Entry[] internal history;

    constructor(address feed, Rules.Limits memory start) {
        oracle = Feed(feed);
        bounds = start;
    }

    function read(uint256 id) external view returns (address) {
        Entry storage entry = entries[id];
        if (entry.amount == 0) revert Rules.OutOfRange(id);
        Entry storage last = history[history.length - 1];
        return entry.stage == Stage.Live ? entry.owner : last.owner;
    }

    function add(uint256 id, uint256 amount) external {
        if (!bounds.within(amount)) revert Rules.OutOfRange(amount);
        entries[id] = Entry({
            owner: msg.sender,
            amount: Rules.scale(amount),
            stage: Stage.Live
        });
        history.push(Entry(msg.sender, amount, Stage.Idle));
    }

    function supports(bytes4 id) external pure returns (bool) {
        return id == type(Feed).interfaceId;
    }

    function spread(Rules.Limits memory given, Caps.Limits memory top) internal pure returns (uint256) {
        Rules.Limits memory copy = given;
        Caps.Limits memory roof = top;
        return copy.high - copy.low + roof.cap;
    }

    /// @notice Sum of the pages.
    // solhint-disable-next-line no-empty-blocks
    function pages(mapping(uint256 => uint256) storage book) internal view returns (uint256) {
        return book[0] + ceiling.cap;
    }
}
