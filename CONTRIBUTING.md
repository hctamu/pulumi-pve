# Contributing Guidelines

Thank you for considering contributing to this project! To keep our workflow clean and consistent, please follow the guidelines below.

---

## Fork-Based Contribution Workflow

1. **Fork** this repository to your own GitHub account.
2. **Clone** your fork to your local machine:
   ```bash
   git clone https://github.com/<your-username>/<repo-name>.git
   ```
3. **Create a new branch** for your work:
   ```bash
   git checkout -b feature/your-feature-name
   ```
4. Make your changes in your branch.
5. **Commit** your changes (see commit message guidelines below).
6. **Push** your branch to your fork on GitHub:
   ```bash
   git push origin feature/your-feature-name
   ```
7. **Open a pull request** from your fork to the main repository.

8. Address any review comments and repeat steps 4–7 as needed.

---

## Branch Naming Convention

Follow these patterns when naming your branches:
- `feature/<feature-name>` — For new features
- `bugfix/<bug-description>` — For bug fixes
- `hotfix/<urgent-fix>` — For critical or urgent fixes
- `release/<version>` — For release-related branches

**Examples:**
- `feature/login-ui`
- `bugfix/typo-in-readme`
- `hotfix/security-patch`
- `release/v1.0.0`

---

## Commit Message Guidelines

Please use the [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>(<scope>): <short description>
```

**Types:**  
- `feat`: New feature  
- `fix`: Bug fix  
- `docs`: Documentation changes  
- `style`: Code style changes (formatting, missing semi colons, etc.)  
- `refactor`: Code refactoring without functional changes  
- `test`: Adding or fixing tests  
- `chore`: Build process or auxiliary tool changes  

**Examples:**
- `feat(auth): add login functionality`
- `fix(ui): resolve overlapping button issue`
- `docs(readme): update contribution section`

---

## Tagging / Release Convention

All tags should follow [Semantic Versioning](https://semver.org/) principles:

- **Production Release:**  
  ```
  v<MAJOR>.<MINOR>.<PATCH>
  ```
  **Example:**  
  `v1.2.3`

- **Release Candidate:**  
  ```
  v<MAJOR>.<MINOR>.<PATCH>-rc<NUMBER>
  ```
  **Example:**  
  `v2.0.0-rc1` (first candidate for the v2.0.0 release)

- **Development Build:**  
  ```
  v<MAJOR>.<MINOR>.<PATCH>-dev<IDENTIFIER>
  ```
  **Example:**  
  `v1.3.0-dev1` (first development/pre-release for v1.3.0)
  or  
  `v1.3.0-dev20240709` (date-based, optional)

**Summary Table**

| Tag Type       | Format                            | Example               |
|----------------|-----------------------------------|-----------------------|
| Release        | vMAJOR.MINOR.PATCH                | v1.2.3                |
| Release Candidate | vMAJOR.MINOR.PATCH-rcNUMBER       | v2.0.0-rc1            |
| Development    | vMAJOR.MINOR.PATCH-devIDENTIFIER  | v1.3.0-dev1           |

---

## Questions?

If you have any questions, open an issue or contact a maintainer.

---

**Thank you for your contribution!**
