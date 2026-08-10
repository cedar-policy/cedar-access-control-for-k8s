# Development

## Contributing Guidelines

Thank you for your interest in contributing to our project. Whether it's a bug report, new feature, correction, or additional
documentation, we greatly value feedback and contributions from our community.

Please read through this document before submitting any issues or pull requests to ensure we have all the necessary
information to effectively respond to your bug report or contribution.

## Reporting Bugs/Feature Requests

We welcome you to use the GitHub issue tracker to report bugs or suggest features.

When filing an issue, please check existing open, or recently closed, issues to make sure somebody else hasn't already
reported the issue. Please try to include as much information as you can. Details like these are incredibly useful:

* A reproducible test case or series of steps
* The version of our code being used
* Any modifications you've made relevant to the bug
* Anything unusual about your environment or deployment


## Contributing via Pull Requests
Contributions via pull requests are much appreciated. Before sending us a pull request, please ensure that:

1. You are working against the latest source on the *main* branch.
2. You check existing open, and recently merged, pull requests to make sure someone else hasn't addressed the problem already.
3. You open an issue to discuss any significant work - we would hate for your time to be wasted.

To send us a pull request, please:

1. Fork the repository.
2. Modify the source; please focus on the specific change you are contributing. If you also reformat all the code, it will be hard for us to focus on your change.
3. Ensure local tests pass.
4. Commit to your fork using clear commit messages.
5. Send us a pull request, answering any default questions in the pull request interface.
6. Pay attention to any automated CI failures reported in the pull request, and stay involved in the conversation.

GitHub provides additional document on [forking a repository](https://help.github.com/articles/fork-a-repo/) and
[creating a pull request](https://help.github.com/articles/creating-a-pull-request/).


## Finding contributions to work on
Looking at the existing issues is a great way to find something to contribute on. As our projects, by default, use the default GitHub issue labels (enhancement/bug/duplicate/help wanted/invalid/question/wontfix), looking at any 'help wanted' issues is a great place to start.


## Code of Conduct
This project follows the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/main/code-of-conduct.md).
For more information see the [Code of Conduct FAQ](https://www.cncf.io/conduct/faq/) or contact
conduct@cncf.io with any additional questions or comments.


## Security issue notifications
If you discover a potential security issue in this project we ask that you notify AWS/Amazon Security via our [vulnerability reporting page](http://aws.amazon.com/security/vulnerability-reporting/). Please do **not** create a public GitHub issue.

## Licensing

See the [LICENSE](LICENSE) file for our project's licensing. We will ask you to confirm the licensing of your contribution.

## TODOs

Initial development needed:
- [ ] Add corresponding flags for hard coded options in `server/options/options.go` (`CedarAuthorizerDefaultCertDir`, `CedarAuthorizerPolicyDir`) so local non-container development is easier to work with
- [ ] Create a validating webhook to validate policy validity
- [ ] RBAC Converter
  - [ ] Update converter test cases to read YAML input from testcase files
  - [ ] Migrate custom policies in tests to YAML testcase files
- [x] Merge authorization and admission webhook into one server with multiple handlers


## Cedar enhancements that would benefit this project

* [Extended has](https://github.com/cedar-policy/rfcs/blob/main/text/0062-extended-has.md): This will simplify `resource has meta && resource.meta has labels && ...` into just `resource.meta.labels has myKey`
* [Datetime extension](https://github.com/cedar-policy/rfcs/blob/main/text/0080-datetime-extension.md): So Cedar can make decisions on timestamp fields like, `authentication::v1::TokenRequest.expirationTimestamp`
* [Enum Entity Types](https://github.com/cedar-policy/rfcs/blob/main/text/0053-enum-entities.md): for Kubernetes API string fields that have limited options
* [Cedar schema annotations](https://github.com/cedar-policy/rfcs/pull/48): To facilitate documentation of schema fields and types
* Cedar schema HTML/Markdown documentation generation
* [Support comparison of records with optional fields](https://github.com/cedar-policy/cedar/issues/1303)

## Converter testing

To run the converter tests, run:
```bash
go test ./internal/convert/...
```

To update the goldenfile outputs in the `internal/convert/testcases/*.cedar`, format them, and validate them, run:
```bash
go test ./internal/convert/... -update=true
make format-policies validate-policies
```
